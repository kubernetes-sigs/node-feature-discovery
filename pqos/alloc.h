/*
 * BSD LICENSE
 *
 * Copyright(c) 2014-2017 Intel Corporation. All rights reserved.
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

/**
 * Allocation module
 */

#include <stdint.h>
#include <stdio.h>
#include "pqos.h"

#ifndef __ALLOCATION_H__
#define __ALLOCATION_H__

#ifdef __cplusplus
extern "C" {
#endif

/**
 * @brief Defines allocation class of service
 *
 * @param [in] arg string passed to -e command line option
 */
void selfn_allocation_class(const char *arg);

/**
 * @brief Associates cores with selected class of service
 *
 * @param [in] arg string passed to -a command line option
 */
void selfn_allocation_assoc(const char *arg);

/**
 * @brief Prints information about cache allocation settings in the system
 *
 * @param [in] cap_mon monitoring capability structure
 * @param [in] cap_l3ca L3 CAT capability structures
 * @param [in] cap_l2ca L2 CAT capability structures
 * @param [in] cap_mba MBA capability structures
 * @param [in] sock_count number of detected CPU sockets
 * @param [in] sockets arrays with detected CPU socket id's
 * @param [in] cpu_info cpu information structure
 * @param [in] verbose verbose mode flag
 */
void alloc_print_config(const struct pqos_capability *cap_mon,
                        const struct pqos_capability *cap_l3ca,
                        const struct pqos_capability *cap_l2ca,
                        const struct pqos_capability *cap_mba,
                        const unsigned sock_count,
                        const unsigned *sockets,
                        const struct pqos_cpuinfo *cpu_info,
                        const int verbose);

/**
 * @brief Applies allocation settings previously selected via
 *        selfn_xxxx() functions
 *
 * @param [in] cap_l3ca CAT capability structures
 * @param [in] cap_l2ca CAT capability structures
 * @param [in] cap_mba MBA capability structures
 * @param [in] cpu cpu information structure
 *
 * @return Operation status
 * @retval 0 there was no new config to apply
 * @retavl 1 there was new config to apply and it went smoothly
 * @retval -1 an error occurred when applying new config
 */
int alloc_apply(const struct pqos_capability *cap_l3ca,
                const struct pqos_capability *cap_l2ca,
                const struct pqos_capability *cap_mba,
                const struct pqos_cpuinfo *cpu);


#ifdef __cplusplus
}
#endif

#endif /* __ALLOCATION_H__ */
