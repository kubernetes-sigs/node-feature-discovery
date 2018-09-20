/*
 * BSD LICENSE
 *
 * Copyright(c) 2016 Intel Corporation. All rights reserved.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 *
 *   * Redistributions of source code must retain the above copyright
 *     notice, this list of conditions and the following disclaimer.
 *   * Redistributions in binary form must reproduce the above copyright
 *     notice, this list of conditions and the following disclaimer in
 *     the documentation and/or other materials provided with the
 *     distribution.
 *   * Neither the name of Intel Corporation nor the names of its
 *     contributors may be used to endorse or promote products derived
 *     from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <stdarg.h>
#include <string.h>
#include <unistd.h>
#include <ctype.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <time.h>
#include <signal.h>
#include <assert.h>

#include <pqos.h>
#include "dlock.h"
#include "tsc.h"

#define DIM(x) (sizeof(x)/sizeof(x[0]))
#define MB (1024 * 1024)

static void *timer_data_ptr = NULL;
static const size_t timer_data_size = 2 * MB;

static void *main_data_ptr = NULL;
static const size_t main_data_size = 96 * MB;

const long long freq_ms = 100;

static struct tsc_prof timer_prof;
static timer_t timerid;

/**
 * @brief Allocates memory block and initializes it with random data
 *
 * This is to avoid any page faults or copy-on-write exceptions later on
 * when measuring cycles.
 *
 * For simplicity, malloc() is used to allocate memory. Ideally special
 * allocator should be used that allocates physically contiguous memory block.
 *
 * @param sz size of memory block in bytes
 *
 * @return Pointer to allocated memory block
 */
static void *init_memory(const size_t sz)
{
        char *p = NULL;
        size_t i;

        if (sz <= 0)
                return NULL;

        p = (char *) malloc(sz);
        if (p == NULL)
                return NULL;

        for (i = 0; i < sz; i += 32)
                p[i] = (char) rand();

        return (void *)p;
}

/**
 * @brief Generates random number for the timer handler procedure
 *
 * This is required as rand() is not thread safe. This dummy implementation
 * computes large number of random numbers, stores them in a table and
 * re-uses them all over again. This is good enough for the purpose of this
 * application.
 *
 * @return Random number value
 */
static int timer_rand(void)
{
        static int _rand_tab[8192]; /* size has to be power of 2 */
        static int _rand_idx = -1;
        int ret;

        /* generate bunch of random numbers */
        if (_rand_idx == -1) {
                unsigned i;

                for (i = 0; i < DIM(_rand_tab); i++)
                        _rand_tab[i] = rand();
                _rand_idx = 0;
        }

        ret = _rand_tab[_rand_idx];
        _rand_idx = (_rand_idx + 1) & (DIM(_rand_tab) - 1);
        return ret;
}

/**
 * @brief Timer handler procedure
 *
 * This is not a realistic workload and it is a demonstration code only.
 *
 * It runs couple thousand of iterations and each iteration is randomizing
 * memory locations to run a number of arithmetic operations on them.
 *
 * @param sig UNUSED
 * @param si UNUSED
 * @param uc UNUSED
 */
static void timer_handler(int sig, siginfo_t *si, void *uc)
{
        const int num_iterations = 5000;
        int *p = (int *) timer_data_ptr;
        const size_t sz = timer_data_size / sizeof(int);
        int m;

        (void) (sig);
        (void) (si);
        (void) (uc);

        tsc_start(&timer_prof);
        /* START - "latency sensitive" code */
        for (m = 0; m < num_iterations; m++) {
                const size_t stride = 5;
                const int idx0 = timer_rand() % (sz - stride);
                const int idx1 = timer_rand() % (sz - stride);
                size_t n;

                for (n = 0; n < stride; n++)
                        p[idx0 + n] = 2 * p[idx1 + n] + p[idx0 + n];
        }
        /* END - "latency sensitive" code */
        tsc_end(&timer_prof, 1);
}

/**
 * @brief Set up the timer
 *
 * The timer expiration is delivered as a signal,
 * the signal handler is timer_handler() above.
 *
 * @param freq_nanosecs timer frequency in nanoseconds
 *
 * @return Operation status
 * @retval 0 OK
 * @retval <0 error
 */
static int init_timer(const long long freq_nanosecs)
{
        sigset_t mask;
        struct sigaction sa;
        struct sigevent sev;
        struct itimerspec its;

        /* this will initialize the table with random numbers */
        (void) timer_rand();

        /* Block timer signal temporarily */
        sigemptyset(&mask);
        sigaddset(&mask, SIGRTMIN);
        if (sigprocmask(SIG_SETMASK, &mask, NULL) == -1) {
                printf("Error masking signal!\n");
                return -1;
        }

        /* set signal handler */
        sa.sa_flags = SA_SIGINFO;
        sa.sa_sigaction = timer_handler;
        sigemptyset(&sa.sa_mask);
        if (sigaction(SIGRTMIN, &sa, NULL) == -1) {
                printf("Error setting signal handler!\n");
                return -1;
        }

        /* Create the timer */
        sev.sigev_notify = SIGEV_SIGNAL;
        sev.sigev_signo = SIGRTMIN;
        sev.sigev_value.sival_ptr = &timerid;
        if (timer_create(CLOCK_REALTIME, &sev, &timerid) == -1) {
                printf("Error creating the timer!\n");
                return -1;
        }

        /* Start the timer */
        its.it_value.tv_sec = freq_nanosecs / 1000000000;
        its.it_value.tv_nsec = freq_nanosecs % 1000000000;
        its.it_interval.tv_sec = freq_nanosecs / 1000000000;
        its.it_interval.tv_nsec = freq_nanosecs % 1000000000;

        if (timer_settime(timerid, 0, &its, NULL) == -1) {
                printf("Error starting the timer!\n");
                return -1;
        }

        /* Unlock the timer signal */
        if (sigprocmask(SIG_UNBLOCK, &mask, NULL) == -1) {
                printf("Error unmasking signal!\n");
                return -1;
        }

        return 0;
}

/**
 * @brief Stop & close the timer
 *
 * @return Operation status
 * @retval 0 OK
 * @retval <0 error
 */
static int close_timer(void)
{
        if (timer_delete(timerid) == -1) {
		printf("Error deleting the timer!\n");
                return -1;
        }
        return 0;
}

/**
 * @brief Initializes PQoS library
 *
 * To satisfy dlock_init() requirements CAT is reset here.
 * More sophisticated solution would be to look for unused CLOS here and
 * pass it on to dlock_init().
 *
 * @return Operation status
 * @retval 0 OK
 * @retval <0 error
 */
static int init_pqos(void)
{
        const struct pqos_cpuinfo *p_cpu = NULL;
        const struct pqos_cap *p_cap = NULL;
	struct pqos_config cfg;
        int ret;

	memset(&cfg, 0, sizeof(cfg));
        cfg.fd_log = STDOUT_FILENO;
        cfg.verbose = 0;
	ret = pqos_init(&cfg);
	if (ret != PQOS_RETVAL_OK) {
		printf("Error initializing PQoS library!\n");
		return -1;
	}

	/* Get CMT capability and CPU info pointer */
	ret = pqos_cap_get(&p_cap, &p_cpu);
	if (ret != PQOS_RETVAL_OK) {
                pqos_fini();
                printf("Error retrieving PQoS capabilities!\n");
		return -1;
	}

        /* Reset CAT */
	ret = pqos_alloc_reset(PQOS_REQUIRE_CDP_ANY);
	if (ret != PQOS_RETVAL_OK) {
                pqos_fini();
		printf("Error resetting CAT!\n");
		return -1;
        }

        return 0;
}

/**
 * @brief Closes PQoS library
 *
 * @return Operation status
 * @retval 0 OK
 * @retval <0 error
 */
static int close_pqos(void)
{
        int ret_val = 0;

	if (pqos_fini() != PQOS_RETVAL_OK) {
		printf("Error shutting down PQoS library!\n");
                ret_val = -1;
        }

        return ret_val;
}

/**
 * @brief Implements memory intensive workload on random locations
 *
 * This is plain memcpy() on random locations and random sizes.
 *
 * @param p pointer to memory block on which the workload is to be run
 * @param size size of the memory block
 */
static void main_thread(char *p, const size_t size)
{
        const size_t half_size = size / 2;
        const unsigned loop_iter = 10000000;
	unsigned i;

        printf("%s() started. please wait ...\n", __func__);

        for (i = 0; i < loop_iter; i++) {
                const size_t copy_size = 6 * 1024;
                const int rnd1 = rand();
                const int rnd2 = rand();
                const size_t si = half_size + rnd1 % (half_size - copy_size);
                const size_t di = rnd2 % (half_size - copy_size);

                memcpy(&p[di], &p[si], copy_size);
        }

        printf("%s() has finished.\n", __func__);
}

/**
 * @brief Parses command line options and implements application logic
 *
 * @param argc number of arguments in the command line
 * @param argv table with command line argument strings
 *
 * @return Process exit code
 */
int main(int argc, char *argv[])
{
        const long long freq_nanosecs = freq_ms * 1000LL * 1000LL;
	int core_id, lock_data = 1, exit_val = EXIT_SUCCESS;

        if (argc < 3) {
                printf("Usage: %s <core_id> <lock|nolock>\n", argv[0]);
                exit(EXIT_FAILURE);
        }

        if (strcasecmp(argv[2], "nolock") != 0 &&
            strcasecmp(argv[2], "lock") != 0) {
                printf("Invalid data lock setting '%s'!\n", argv[2]);
                printf("Usage: %s <core_id> <lock|nolock>\n", argv[0]);
                exit(EXIT_FAILURE);
        }

        core_id = atoi(argv[1]);
        lock_data = (strcasecmp(argv[2], "nolock") == 0) ? 0 : 1;

        /* allocate memory blocks */
        main_data_ptr = init_memory(main_data_size);
        timer_data_ptr = init_memory(timer_data_size);
        if (main_data_ptr == NULL || timer_data_ptr == NULL) {
                exit_val = EXIT_FAILURE;
                goto error_exit1;
        }

        if (lock_data) {
                /* initialize PQoS and lock the data */
                if (init_pqos() != 0) {
                        exit_val = EXIT_FAILURE;
                        goto error_exit1;
                }

                /* lock the timer data */
                if (dlock_init(timer_data_ptr,
                               timer_data_size, 1 /* CLOS */, core_id) != 0) {
                        printf("Pseudo data lock error!\n");
                        exit_val = EXIT_FAILURE;
                        goto error_exit1;
                }
        }

        tsc_init(&timer_prof, "Timer Handler");

        if (init_timer(freq_nanosecs) != 0) {
                printf("Timer start error!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit2;
        }

        main_thread((char *)main_data_ptr, main_data_size);

        (void) close_timer();

        tsc_print(&timer_prof);

 error_exit2:
        if (lock_data)
                dlock_exit();

 error_exit1:
        if (lock_data)
                (void) close_pqos();

        if (main_data_ptr != NULL)
                free(main_data_ptr);
        if (timer_data_ptr != NULL)
                free(timer_data_ptr);
	return exit_val;
}
