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
 * @brief Platform QoS sample COS association application
 *
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <ctype.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include "pqos.h"

/**
 * Defines
 */
#define PQOS_MAX_CORES        (1024)

/**
 * Number of cores selected for cache allocation association
 */
static int sel_l3ca_assoc_num = 0;

/**
 * Maintains a table of core and class_id that are selected in config string for
 * setting up allocation policy per core
 */
static struct {
        unsigned core;
        unsigned class_id;
} sel_l3ca_assoc_tab[PQOS_MAX_CORES];

/**
 * @brief Verifies and translates association config string into
 *        internal configuration.
 *
 * @param argc Number of arguments in input command
 * @param argv Input arguments for COS association
 */
static void
enforcement_get_input(int argc, char *argv[])
{
	int i = 0;

	if (argc < 2)
		sel_l3ca_assoc_num = 0;
	else if (!strcmp(argv[1], "-h") || !strcmp(argv[1], "-H")) {
		printf("Usage: %s [<COS#> <core1> <core2> <core3> ...]\n",
                       argv[0]);
                printf("Eg   : %s 1 1 3 6\n\n", argv[0]);
		sel_l3ca_assoc_num = 0;
	} else {
		for (i = 0; i < argc-2; i++) {
			sel_l3ca_assoc_tab[i].class_id =
                                (unsigned) atoi(argv[1]);
			sel_l3ca_assoc_tab[i].core =
                                (unsigned) atoi(argv[i+2]);
		}
		sel_l3ca_assoc_num = (int) i;
	}
}

/**
 * @brief Prints information about cache allocation settings in the system
 */
static void
print_allocation_config(void)
{
	int ret;
	unsigned i;
	unsigned sock_count, *sockets = NULL;
	const struct pqos_cpuinfo *p_cpu = NULL;
	const struct pqos_cap *p_cap = NULL;

	/* Get CMT capability and CPU info pointer */
	ret = pqos_cap_get(&p_cap, &p_cpu);
	if (ret != PQOS_RETVAL_OK) {
		printf("Error retrieving PQoS capabilities!\n");
		return;
	}
	/* Get CPU socket information to set COS */
	sockets = pqos_cpu_get_sockets(p_cpu, &sock_count);
	if (sockets == NULL) {
		printf("Error retrieving CPU socket information!\n");
		return;
	}
	for (i = 0; i < sock_count; i++) {
		unsigned *lcores = NULL;
		unsigned lcount = 0, n = 0;

		lcores = pqos_cpu_get_cores(p_cpu, sockets[i], &lcount);
		if (lcores == NULL || lcount == 0) {
			printf("Error retrieving core information!\n");
                        free(sockets);
			return;
		}
		printf("Core information for socket %u:\n",
				sockets[i]);
		for (n = 0; n < lcount; n++) {
			unsigned class_id = 0;

			ret = pqos_alloc_assoc_get(lcores[n], &class_id);
			if (ret == PQOS_RETVAL_OK)
				printf("    Core %u => COS%u\n",
                                       lcores[n], class_id);
			else
				printf("    Core %u => ERROR\n",
                                       lcores[n]);
		}
                free(lcores);
	}
        free(sockets);
}
/**
 * @brief Sets up association between cores and allocation classes of service
 *
 * @return Number of associations made
 * @retval 0 no association made (nor requested)
 * @retval negative error
 * @retval positive sucess
 */
static int
set_allocation_assoc(void)
{
	int i;

	for (i = 0; i < sel_l3ca_assoc_num; i++) {
                int ret;

		ret = pqos_alloc_assoc_set(sel_l3ca_assoc_tab[i].core,
                                           sel_l3ca_assoc_tab[i].class_id);
		if (ret != PQOS_RETVAL_OK) {
			printf("Setting allocation class of service "
                               "association failed!\n");
			return -1;
		}
	}
	return sel_l3ca_assoc_num;
}

int main(int argc, char *argv[])
{
	struct pqos_config cfg;
	int ret, exit_val = EXIT_SUCCESS;

	memset(&cfg, 0, sizeof(cfg));
        cfg.fd_log = STDOUT_FILENO;
        cfg.verbose = 0;
	/* PQoS Initialization - Check and initialize CAT and CMT capability */
	ret = pqos_init(&cfg);
	if (ret != PQOS_RETVAL_OK) {
		printf("Error initializing PQoS library!\n");
		exit_val = EXIT_FAILURE;
		goto error_exit;
	}

	/* Get input from user */
	enforcement_get_input(argc, argv);
	if (sel_l3ca_assoc_num) {
		/* Enforce COS to the associated cores */
		ret = set_allocation_assoc();
		if (ret < 0) {
			printf("CAT association error!\n");
			goto error_exit;
		}
		printf("Allocation configuration altered.\n");
	}
	/* Print COS and associated cores */
	print_allocation_config();
error_exit:
	/* reset and deallocate all the resources */
	ret = pqos_fini();
	if (ret != PQOS_RETVAL_OK)
		printf("Error shutting down PQoS library!\n");
	return exit_val;
}
