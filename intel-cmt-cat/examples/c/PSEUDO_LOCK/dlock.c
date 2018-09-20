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
#include <stdint.h>   /* uint64_t etc. */
#include <stdlib.h>   /* malloc() */
#include <string.h>   /* memcpy() */
#include <pqos.h>
#ifdef __linux__
#include <sched.h>    /* sched_setaffinity() */
#endif
#ifdef __FreeBSD__
#include <sys/param.h>   /* sched affinity */
#include <sys/cpuset.h>  /* sched affinity */
#endif
#include "dlock.h"

#define MAX_SOCK_NUM 16
#define DIM(x) (sizeof(x)/sizeof(x[0]))

static int m_is_chunk_allocated = 0;
static char *m_chunk_start = NULL;
static size_t m_chunk_size = 0;
static unsigned m_num_clos = 0;
static struct {
        unsigned id;
        struct pqos_l3ca *cos_tab;
} m_socket_cos[MAX_SOCK_NUM];

/**
 * @brief Removes the memory block from cache hierarchy
 *
 * @param p pointer to memory block
 * @param s size of memory block in bytes
 */
static void mem_flush(const void *p, const size_t s)
{
        const size_t cache_line = 64;
        const char *cp = (const char *)p;
        size_t i = 0;

        if (p == NULL || s <= 0)
                return;

        for (i = 0; i < s; i += cache_line) {
                asm volatile("clflush (%0)\n\t"
                             :
                             : "r"(&cp[i])
                             : "memory");
        }

        asm volatile("sfence\n\t"
                     :
                     :
                     : "memory");
}

/**
 * @brief Reads the memory block which places it in cache hierarchy
 *
 * @param p pointer to memory block
 * @param s size of memory block in bytes
 */
static void mem_read(const void *p, const size_t s)
{
        register size_t i;

        if (p == NULL || s <= 0)
                return;

        for (i = 0; i < (s / sizeof(uint32_t)); i++) {
                asm volatile("xor (%0,%1,4), %%eax\n\t"
                             :
                             : "r" (p), "r" (i)
                             : "%eax", "memory");
        }

        for (i = s & (~(sizeof(uint32_t) - 1)); i < s; i++) {
                asm volatile("xorb (%0,%1,1), %%al\n\t"
                             :
                             : "r" (p), "r" (i)
                             : "%al", "memory");
        }
}

/**
 * @brief Initializes the memory block with random data
 *
 * This is to avoid any page faults or copy-on-write exceptions later on.
 *
 * @param p pointer to memory block
 * @param s size of memory block in bytes
 */
static void mem_init(void *p, const size_t s)
{
        char *cp = (char *)p;
        size_t i;

        if (p == NULL || s <= 0)
                return;

        for (i = 0; i < s; i++)
                cp[i] = (char) rand();
}

/**
 * @brief Calculates number of cache ways required to fit a number of \a bytes
 *
 * @param cat_cap pointer to L3CA PQoS capability structure
 * @param bytes number of bytes
 * @param ways pointer to store number of required cache ways
 *
 * @return Operation status
 * @retval 0 OK
 * @retval <0 error
 */
static int bytes_to_cache_ways(const struct pqos_capability *cat_cap,
                               const size_t bytes, size_t *ways)
{
        size_t llc_size = 0, num_ways = 0;
        const struct pqos_cap_l3ca *cat = NULL;

        if (cat_cap == NULL || ways == NULL)
                return -1;

        if (cat_cap->type != PQOS_CAP_TYPE_L3CA)
                return -2;

        cat = cat_cap->u.l3ca;
        llc_size = cat->way_size * cat->num_ways;
        if (bytes > llc_size)
                return -3;

        num_ways = (bytes + cat->way_size - 1) / cat->way_size;
        if (num_ways >= cat->num_ways)
                return -4;

        if (num_ways < 2)
                num_ways = 2;

        *ways = num_ways;
        return 0;
}

int dlock_init(void *ptr, const size_t size, const int clos, const int cpuid)
{
	const struct pqos_cpuinfo *p_cpu = NULL;
	const struct pqos_cap *p_cap = NULL;
        const struct pqos_capability *p_l3ca_cap = NULL;
        unsigned *sockets = NULL;
        unsigned socket_count = 0, i = 0;
        int ret = 0, res = 0;
#ifdef __linux__
        cpu_set_t cpuset_save, cpuset;
#endif
#ifdef __FreeBSD__
        cpuset_t cpuset_save, cpuset;
#endif

        if (m_chunk_start != NULL)
                return -1;

        if (size <= 0)
                return -2;

        if (ptr != NULL) {
                m_chunk_start = ptr;
                m_is_chunk_allocated = 0;
        } else {
                /**
                 * For best results allocated memory should be physically
                 * contiguous. Yet this would require allocating memory in
                 * kernel space or using huge pages.
                 * Let's use malloc() and 4K pages for simplicity.
                 */
                m_chunk_start = malloc(size);
                if (m_chunk_start == NULL)
                        return -3;
                m_is_chunk_allocated = 1;
                mem_init(m_chunk_start, size);
        }
        m_chunk_size = size;

        /**
         * Get task affinity to restore it later
         */
#ifdef __linux__
        res = sched_getaffinity(0, sizeof(cpuset_save), &cpuset_save);
#endif
#ifdef __FreeBSD__
        res = cpuset_getaffinity(CPU_LEVEL_WHICH, CPU_WHICH_TID, -1,
                                 sizeof(cpuset_save), &cpuset_save);
#endif
        if (res != 0) {
                perror("dlock_init() error");
                ret = -4;
                goto dlock_init_error1;
        }

        /**
         * Set task affinity to cpuid for data locking phase
         */
        CPU_ZERO(&cpuset);
        CPU_SET(cpuid, &cpuset);
#ifdef __linux__
        res = sched_setaffinity(0, sizeof(cpuset), &cpuset);
#endif
#ifdef __FreeBSD__
        res = cpuset_setaffinity(CPU_LEVEL_WHICH, CPU_WHICH_TID, -1,
                                 sizeof(cpuset), &cpuset);
#endif
        if (res != 0) {
                perror("dlock_init() error");
                ret = -4;
                goto dlock_init_error1;
        }

        /**
         * Clear table for restoring CAT configuration
         */
        for (i = 0; i < DIM(m_socket_cos); i++) {
                m_socket_cos[i].id = 0;
                m_socket_cos[i].cos_tab = NULL;
        }

        /**
         * Retrieve CPU topology and PQoS capabilities
         */
	res = pqos_cap_get(&p_cap, &p_cpu);
	if (res != PQOS_RETVAL_OK) {
		ret = -5;
		goto dlock_init_error2;
	}

        /**
         * Retrieve list of CPU sockets
         */
	sockets = pqos_cpu_get_sockets(p_cpu, &socket_count);
	if (sockets == NULL) {
		ret = -6;
		goto dlock_init_error2;
	}

        /**
         * Get CAT capability structure
         */
        res = pqos_cap_get_type(p_cap, PQOS_CAP_TYPE_L3CA, &p_l3ca_cap);
        if (res != PQOS_RETVAL_OK) {
		ret = -7;
		goto dlock_init_error2;
        }

        /**
         * Compute number of cache ways required for the data
         */
        size_t num_cache_ways = 0;

        res = bytes_to_cache_ways(p_l3ca_cap, size, &num_cache_ways);
        if (res != 0) {
		ret = -8;
		goto dlock_init_error2;
        }

        /**
         * Compute class bit mask for data lock and
         * retrieve number of classes of service
         */
        m_num_clos = p_l3ca_cap->u.l3ca->num_classes;

        for (i = 0; i < socket_count; i++) {
                /**
                 * This would be enough to run the below code for the socket
                 * corresponding to \a cpuid. Yet it is safer to keep CLOS
                 * definitions coherent across sockets.
                 */
                const uint64_t dlock_mask = (1ULL << num_cache_ways) - 1ULL;
                struct pqos_l3ca cos[m_num_clos];
                unsigned num = 0, j;

                /* get current CAT classes on this socket */
                res = pqos_l3ca_get(sockets[i], m_num_clos, &num, &cos[0]);
                if (res != PQOS_RETVAL_OK) {
                        printf("pqos_l3ca_get() error!\n");
                        ret = -9;
                        goto dlock_init_error2;
                }

                /* paranoia check */
                if (m_num_clos != num) {
                        printf("CLOS number mismatch!\n");
                        ret = -9;
                        goto dlock_init_error2;
                }

                /* save CAT classes to restore it later */
                m_socket_cos[i].id = sockets[i];
                m_socket_cos[i].cos_tab = malloc(m_num_clos * sizeof(cos[0]));
                if (m_socket_cos[i].cos_tab == NULL) {
                        printf("malloc() error!\n");
                        ret = -9;
                        goto dlock_init_error2;
                }
                memcpy(m_socket_cos[i].cos_tab, cos,
                       m_num_clos * sizeof(cos[0]));

                /**
                 * Modify the classes in the following way:
                 * if class_id == clos then
                 *   set class mask so that it has exclusive access to
                 *   \a num_cache_ways
                 * else
                 *   exclude class from accessing \a num_cache_ways
                 */
                for (j = 0; j < m_num_clos; j++) {
                        if (cos[j].cdp) {
                                if (cos[j].class_id == (unsigned)clos) {
                                        cos[j].u.s.code_mask = dlock_mask;
                                        cos[j].u.s.data_mask = dlock_mask;
                                } else {
                                        cos[j].u.s.code_mask &= ~dlock_mask;
                                        cos[j].u.s.data_mask &= ~dlock_mask;
                                }
                        } else {
                                if (cos[j].class_id == (unsigned)clos)
                                        cos[j].u.ways_mask = dlock_mask;
                                else
                                        cos[j].u.ways_mask &= ~dlock_mask;
                        }
                }

                res = pqos_l3ca_set(sockets[i], m_num_clos, &cos[0]);
                if (res != PQOS_RETVAL_OK) {
                        printf("pqos_l3ca_set() error!\n");
                        ret = -10;
                        goto dlock_init_error2;
                }
        }

        /**
         * Read current cpuid CLOS association and set the new one
         */
        unsigned clos_save = 0;

        res = pqos_alloc_assoc_get(cpuid, &clos_save);
        if (res != PQOS_RETVAL_OK) {
                printf("pqos_alloc_assoc_get() error!\n");
                ret = -11;
                goto dlock_init_error2;
        }
        res = pqos_alloc_assoc_set(cpuid, clos);
        if (res != PQOS_RETVAL_OK) {
                printf("pqos_alloc_assoc_set() error!\n");
                ret = -12;
                goto dlock_init_error2;
        }

        /**
         * Remove buffer data from cache hierarchy and read it back into
         * selected cache ways.
         * WBINVD is another option to remove data from cache but it is
         * privileged instruction and as such has to be done in kernel space.
         */
        mem_flush(m_chunk_start, m_chunk_size);

        /**
         * Read the data couple of times. This may help as this is ran in
         * user space and code can be interrupted and data removed
         * from cache hierarchy.
         * Ideally all locking should be done at privileged level with
         * full system control.
         */
        for (i = 0; i < 10; i++)
                mem_read(m_chunk_start, m_chunk_size);

        /**
         * Restore cpuid clos association
         */
        res = pqos_alloc_assoc_set(cpuid, clos_save);
        if (res != PQOS_RETVAL_OK) {
                printf("pqos_alloc_assoc_set() error (revert)!\n");
                ret = -13;
                goto dlock_init_error2;
        }

 dlock_init_error2:
        for (i = 0; (i < DIM(m_socket_cos)) && (ret != 0); i++)
                if (m_socket_cos[i].cos_tab != NULL)
                        free(m_socket_cos[i].cos_tab);

#ifdef __linux__
        res = sched_setaffinity(0, sizeof(cpuset_save), &cpuset_save);
#endif
#ifdef __FreeBSD__
        res = cpuset_setaffinity(CPU_LEVEL_WHICH, CPU_WHICH_TID, -1,
                                 sizeof(cpuset_save), &cpuset_save);
#endif
        if (res != 0)
                perror("dlock_init() error restoring affinity");

 dlock_init_error1:
        if (m_is_chunk_allocated && ret != 0)
                free(m_chunk_start);

        if (ret != 0) {
                m_chunk_start = NULL;
                m_chunk_size = 0;
                m_is_chunk_allocated = 0;
        }

        if (sockets != NULL)
                free(sockets);

        return ret;
}

int dlock_exit(void)
{
        int ret = 0;
        unsigned i;

        if (m_chunk_start == NULL)
                return -1;

        for (i = 0; i < DIM(m_socket_cos); i++) {
                if (m_socket_cos[i].cos_tab != NULL) {
                        int res = pqos_l3ca_set(m_socket_cos[i].id, m_num_clos,
                                                m_socket_cos[i].cos_tab);

                        if (res != PQOS_RETVAL_OK)
                                ret = -2;
                }
                free(m_socket_cos[i].cos_tab);
                m_socket_cos[i].cos_tab = NULL;
                m_socket_cos[i].id = 0;
        }

        if (m_is_chunk_allocated)
                free(m_chunk_start);

        m_chunk_start = NULL;
        m_chunk_size = 0;
        m_is_chunk_allocated = 0;

        return ret;
}
