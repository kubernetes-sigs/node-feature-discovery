/*
 * BSD LICENSE
 *
 * Copyright(c) 2014-2016 Intel Corporation. All rights reserved.
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
 *
 */

/**
 * @brief Platform QoS sample LLC occupancy monitoring application
 *
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <signal.h>
#include "pqos.h"

/**
 * Defines
 */
#define PQOS_MAX_CORES        1024
#define PQOS_MAX_PIDS         16
#define PQOS_MAX_MON_EVENTS   1

/**
 * Number of cores that are selected in config string
 * for monitoring LLC occupancy
 */
static int sel_monitor_num = 0;

/**
 * The mask to tell which events to display
 */
static enum pqos_mon_event sel_events_max = 0;

/**
 * Maintains a table of core, event, number of events that are
 * selected in config string for monitoring
 */
static struct {
        unsigned core;
        struct pqos_mon_data *pgrp;
        enum pqos_mon_event events;
} sel_monitor_core_tab[PQOS_MAX_CORES];
static struct pqos_mon_data *m_mon_grps[PQOS_MAX_CORES];

/**
 * Maintains a table of process id, event, number of events that are selected
 * in config string for monitoring LLC occupancy
 */
static struct {
        pid_t pid;
        struct pqos_mon_data *pgrp;
        enum pqos_mon_event events;
} sel_monitor_pid_tab[PQOS_MAX_PIDS];

/**
 * Maintains the number of process id's you want to track
 */
static int sel_process_num = 0;

/**
 * Flag to determine which library interface to use
 */
static int interface = PQOS_INTER_MSR;

static void stop_monitoring(void);

/**
 * @brief CTRL-C handler for infinite monitoring loop
 *
 * @param [in] signo signal number
 */
static void monitoring_ctrlc(int signo)
{
	printf("\nExiting[%d]...\n", signo);
        stop_monitoring();
        if (pqos_fini() != PQOS_RETVAL_OK) {
		printf("Error shutting down PQoS library!\n");
                exit(EXIT_FAILURE);
        }
	exit(EXIT_SUCCESS);
}

/**
 * @brief Scale byte value up to KB
 *
 * @param [in] bytes value to be scaled up
 * @return scaled up value in KB's
 */
static inline double bytes_to_kb(const double bytes)
{
        return bytes / 1024.0;
}

/**
 * @brief Scale byte value up to MB
 *
 * @param [in] bytes value to be scaled up
 * @return scaled up value in MB's
 */
static inline double bytes_to_mb(const double bytes)
{
        return bytes / (1024.0 * 1024.0);
}

/**
 * @brief Check to determine if processes or cores are monitored
 *
 * @return Process monitoring mode status
 * @retval 0 monitoring cores
 * @retval 1 monitoring processes
 */
static inline int process_mode(void)
{
        return (sel_process_num <= 0) ? 0 : 1;
}

/**
 * @brief Verifies and translates monitoring config string into
 *        internal monitoring configuration.
 *
 * @param [in] argc Number of arguments in input command
 * @param [in] argv Input arguments for LLC occupancy monitoring
 */
static void
monitoring_get_input(int argc, char *argv[])
{
	int num_args, num_opts = 1, i = 0, sel_pid = 0, help = 0;

        for (i = 0; i < argc; i++) {
                if (!strcmp(argv[i], "-p")) {
                        sel_pid = 1;
                        num_opts++;
                } else if (!strcmp(argv[i], "-I")) {
                        interface = PQOS_INTER_OS;
                        num_opts++;
                } else if (!strcmp(argv[i], "-H") || !strcmp(argv[i], "-h")) {
                        help = 1;
                        num_opts++;
                }
        }
        /* Ensure OS interface selected if monitoring tasks */
        if (sel_pid && interface == PQOS_INTER_MSR) {
                printf("Error: PID monitoring requires OS interface "
                        "selection!\nPlease use the -I option.\n");
                help = 1;
        }
        num_args = (argc - num_opts);
        if (help) {
		printf("Usage:  %s [<core1> <core2> <core3> ...]\n"
                       "        %s -I -p [<pid1> <pid2> <pid3> ...]\n",
                       argv[0], argv[0]);
		printf("Eg   :  %s 1 2 6\n        "
                       "%s -I -p 3564 7638 356\n"
                       "Notes:\n        "
                       "-h      help\n        "
                       "-I      select library OS interface\n        "
                       "-p      select process ID's to monitor LLC occupancy"
                       "\n\n", argv[0], argv[0]);
		exit(EXIT_SUCCESS);
        } else if (num_args == 0) {
		sel_monitor_num = 0;
        } else {
                if (sel_pid) {
                        if (num_args > PQOS_MAX_PIDS)
                                num_args = PQOS_MAX_PIDS;
                        for (i = 0; i < num_args; i++) {
                                m_mon_grps[i] = malloc(sizeof(**m_mon_grps));
                                sel_monitor_pid_tab[i].pgrp = m_mon_grps[i];
                                sel_monitor_pid_tab[i].pid =
                                        (unsigned) atoi(argv[num_opts + i]);
                        }
                        sel_process_num = (int) num_args;
                } else {
                        if (num_args > PQOS_MAX_CORES)
                                num_args = PQOS_MAX_CORES;
                        for (i = 0; i < num_args; i++) {
                                m_mon_grps[i] = malloc(sizeof(**m_mon_grps));
                                sel_monitor_core_tab[i].pgrp = m_mon_grps[i];
                                sel_monitor_core_tab[i].core =
                                        (unsigned) atoi(argv[num_opts + i]);
                        }
                        sel_monitor_num = (int) num_args;
                }
	}
}

/**
 * @brief Starts monitoring on selected cores/PIDs
 *
 * @param [in] cpu_info cpu information structure
 * @param [in] cap_mon monitoring capabilities structure
 *
 * @return Operation status
 * @retval 0 OK
 * @retval -1 error
 */
static int
setup_monitoring(const struct pqos_cpuinfo *cpu_info,
                 const struct pqos_capability * const cap_mon)
{
	unsigned i;
        const enum pqos_mon_event perf_events =
                PQOS_PERF_EVENT_IPC | PQOS_PERF_EVENT_LLC_MISS;

        for (i = 0; (unsigned)i < cap_mon->u.mon->num_events; i++)
                sel_events_max |= (cap_mon->u.mon->events[i].type);

        /* Remove perf events IPC and LLC MISSES */
        sel_events_max &= ~perf_events;
        if (sel_monitor_num == 0 && sel_process_num == 0) {
                for (i = 0; i < cpu_info->num_cores; i++) {
                        unsigned lcore = cpu_info->cores[i].lcore;

                        sel_monitor_core_tab[sel_monitor_num].core = lcore;
                        sel_monitor_core_tab[sel_monitor_num].events =
                                sel_events_max;
                        m_mon_grps[sel_monitor_num] =
                                malloc(sizeof(**m_mon_grps));
                        sel_monitor_core_tab[sel_monitor_num].pgrp =
                                m_mon_grps[sel_monitor_num];
                        sel_monitor_num++;
                }
        }
        if (!process_mode()) {
                for (i = 0; i < (unsigned) sel_monitor_num; i++) {
                        unsigned lcore = sel_monitor_core_tab[i].core;
                        int ret;

                        ret = pqos_mon_start(1, &lcore,
                                             sel_events_max,
                                             NULL,
                                             sel_monitor_core_tab[i].pgrp);
                        if (ret != PQOS_RETVAL_OK) {
                                printf("Monitoring start error on core %u,"
                                       "status %d\n", lcore, ret);
                                return ret;
                        }
                }
        } else {
                for (i = 0; i < (unsigned) sel_process_num; i++) {
                        pid_t pid = sel_monitor_pid_tab[i].pid;
                        int ret;

                        ret = pqos_mon_start_pid(pid, PQOS_MON_EVENT_L3_OCCUP,
                                                 NULL,
                                                 sel_monitor_pid_tab[i].pgrp);
                        if (ret != PQOS_RETVAL_OK) {
                                printf("Monitoring start error on pid %u,"
                                       "status %d\n", pid, ret);
                                return ret;
                        }
                }
        }
	return PQOS_RETVAL_OK;
}

/**
 * @brief Stops monitoring on selected cores
 *
 */
static void stop_monitoring(void)
{
	unsigned i, mon_number = 0;

        if (!process_mode())
                mon_number = (unsigned) sel_monitor_num;
        else
                mon_number = (unsigned) sel_process_num;

	for (i = 0; i < mon_number; i++) {
                int ret;

		ret = pqos_mon_stop(m_mon_grps[i]);
		if (ret != PQOS_RETVAL_OK)
			printf("Monitoring stop error!\n");
                free(m_mon_grps[i]);
	}
}

/**
 * @brief Reads monitoring event data
 */
static void monitoring_loop(void)
{
        unsigned mon_number = 0;
	int ret = PQOS_RETVAL_OK;
	int i = 0;

	if (signal(SIGINT, monitoring_ctrlc) == SIG_ERR)
		printf("Failed to catch SIGINT!\n");

        if (!process_mode())
	        mon_number = (unsigned) sel_monitor_num;
	else
	        mon_number = (unsigned) sel_process_num;

	while (1) {
                ret = pqos_mon_poll(m_mon_grps, (unsigned)mon_number);
                if (ret != PQOS_RETVAL_OK) {
                        printf("Failed to poll monitoring data!\n");
                        return;
                }
                if (!process_mode()) {
                        printf("    CORE     RMID    LLC[KB]"
                               "    MBL[MB]    MBR[MB]\n");
                        for (i = 0; i < sel_monitor_num; i++) {
                                const struct pqos_event_values *pv =
                                        &m_mon_grps[i]->values;
                                double llc = bytes_to_kb(pv->llc);
                                double mbr = bytes_to_mb(pv->mbm_remote_delta);
                                double mbl = bytes_to_mb(pv->mbm_local_delta);

                                if (interface == PQOS_INTER_OS)
                                        printf("%8u %s %10.1f %10.1f %10.1f\n",
                                               m_mon_grps[i]->cores[0],
                                               "     N/A", llc, mbl, mbr);
                                else
                                        printf("%8u %8u %10.1f %10.1f %10.1f\n",
                                               m_mon_grps[i]->cores[0],
                                               m_mon_grps[i]->poll_ctx[0].rmid,
                                               llc, mbl, mbr);
                        }
                } else {
                        printf("PID       LLC[KB]\n");
                        for (i = 0; i < sel_process_num; i++) {
                                const struct pqos_event_values *pv =
                                        &m_mon_grps[i]->values;
                                double llc = bytes_to_kb(pv->llc);

                                printf("%6d %10.1f\n",
                                       m_mon_grps[i]->pid, llc);
                        }
                }
		printf("\nPress Enter to continue or Ctrl+c to exit");
		if (getchar() != '\n')
			break;
		printf("\e[1;1H\e[2J");
	}
}

int main(int argc, char *argv[])
{
	struct pqos_config config;
	const struct pqos_cpuinfo *p_cpu = NULL;
	const struct pqos_cap *p_cap = NULL;
	int ret, exit_val = EXIT_SUCCESS;
	const struct pqos_capability *cap_mon = NULL;

        /* Get input from user */
	monitoring_get_input(argc, argv);

        memset(&config, 0, sizeof(config));
        config.fd_log = STDOUT_FILENO;
        config.verbose = 0;
        config.interface = interface;

	/* PQoS Initialization - Check and initialize CAT and CMT capability */
	ret = pqos_init(&config);
	if (ret != PQOS_RETVAL_OK) {
		printf("Error initializing PQoS library!\n");
		exit_val = EXIT_FAILURE;
		goto error_exit;
	}
	/* Get CMT capability and CPU info pointer */
	ret = pqos_cap_get(&p_cap, &p_cpu);
	if (ret != PQOS_RETVAL_OK) {
		printf("Error retrieving PQoS capabilities!\n");
		exit_val = EXIT_FAILURE;
		goto error_exit;
	}
	(void) pqos_cap_get_type(p_cap, PQOS_CAP_TYPE_MON, &cap_mon);
	/* Setup the monitoring resources */
	ret = setup_monitoring(p_cpu, cap_mon);
	if (ret != PQOS_RETVAL_OK) {
		printf("Error Setting up monitoring!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit;
        }
	/* Start Monitoring */
	monitoring_loop();
	/* Stop Monitoring */
	stop_monitoring();
 error_exit:
	ret = pqos_fini();
	if (ret != PQOS_RETVAL_OK)
		printf("Error shutting down PQoS library!\n");
	return exit_val;
}
