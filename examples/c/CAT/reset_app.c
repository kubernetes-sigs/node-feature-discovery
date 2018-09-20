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
 * @brief Platform QoS sample COS reset application
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
 * @brief Prints information about cache allocation settings in the system
 *
 * @param sock_count number of detected CPU sockets
 * @param sockets arrays with detected CPU socket id's
 * @param cpu_info cpu information structure
 */
static void
print_allocation_config(const struct pqos_capability *cap_l3ca,
                        const unsigned sock_count,
                        const unsigned *sockets,
                        const struct pqos_cpuinfo *cpu_info)
{
	int ret;
	unsigned i;

	if (cap_l3ca == NULL)
                return;

        for (i = 0; i < sock_count; i++) {
                struct pqos_l3ca tab[PQOS_MAX_L3CA_COS];
                unsigned num = 0;

                ret = pqos_l3ca_get(sockets[i], PQOS_MAX_L3CA_COS,
                                    &num, tab);
                if (ret == PQOS_RETVAL_OK) {
                        unsigned n = 0;

                        printf("L3CA COS definitions for Socket %u:\n",
                               sockets[i]);
                        for (n = 0; n < num; n++) {
                                printf("   L3CA COS%u => MASK 0x%llx\n",
                                       tab[n].class_id,
                                       (unsigned long long)tab[n].u.ways_mask);
                        }
                }
        }

	for (i = 0; i < sock_count; i++) {
		unsigned *lcores = NULL;
		unsigned lcount = 0, n = 0;

		lcores = pqos_cpu_get_cores(cpu_info, sockets[i], &lcount);
		if (lcores == NULL || lcount == 0) {
			printf("Error retrieving core information!\n");
                        free(lcores);
			return;
		}
		printf("Core information for socket %u:\n",
                       sockets[i]);
		for (n = 0; n < lcount; n++) {
			unsigned class_id = 0;
			int ret1 = PQOS_RETVAL_OK;

			if (cap_l3ca != NULL)
				ret1 = pqos_alloc_assoc_get(lcores[n],
                                                            &class_id);
			if (ret1 == PQOS_RETVAL_OK)
				printf("    Core %u => COS%u\n",
                                       lcores[n], class_id);
			else
				printf("    Core %u => ERROR\n",
                                       lcores[n]);
		}
                free(lcores);
	}
}

int main(int argc, char *argv[])
{
        struct pqos_config cfg;
        const struct pqos_cpuinfo *p_cpu = NULL;
        const struct pqos_cap *p_cap = NULL;
        const struct pqos_capability *cap_l3ca = NULL;
        unsigned sock_count, *sockets = NULL;
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
	/* Get CMT capability and CPU info pointer */
	ret = pqos_cap_get(&p_cap, &p_cpu);
        if (ret != PQOS_RETVAL_OK) {
                printf("Error retrieving PQoS capabilities!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit;
        }
	if (argc == 2 && (!strcmp(argv[1], "-h") || !strcmp(argv[1], "-H"))) {
		printf("Usage: %s\n\n", argv[0]);
		goto error_exit;
	}
	/* Reset Api */
	if (pqos_alloc_reset(PQOS_REQUIRE_CDP_ANY) != PQOS_RETVAL_OK)
		printf("CAT reset failed!\n");
	else
		printf("CAT reset successful\n");
	/* Get CPU socket information to set COS */
	sockets = pqos_cpu_get_sockets(p_cpu, &sock_count);
        if (sockets == NULL) {
                printf("Error retrieving CPU socket information!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit;
        }
	(void) pqos_cap_get_type(p_cap, PQOS_CAP_TYPE_L3CA, &cap_l3ca);
	/* Print COS and associated cores */
	print_allocation_config(cap_l3ca, sock_count, sockets, p_cpu);
 error_exit:
	/* reset and deallocate all the resources */
	ret = pqos_fini();
	if (ret != PQOS_RETVAL_OK)
		printf("Error shutting down PQoS library!\n");
        if (sockets != NULL)
                free(sockets);
	return exit_val;
}
