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
 * @brief CPU sockets and cores enumeration module.
 */

#ifndef __PQOS_CPUINFO_H__
#define __PQOS_CPUINFO_H__

#include <errno.h>
#include "pqos.h"

#ifdef __cplusplus
extern "C" {
#endif

/**
 * @brief Initializes CPU information module
 *
 * CPU topology detection method is OS dependant.
 *
 * @param [out] topology place to store pointer to CPU topology data
 *
 * @return Operation status
 * @retval 0 success
 * @retval -EINVAL invalid argument
 * @retval -EPERM cpuinfo already initialized
 * @retval -EFAULT error building & discovering the topology
 */
int cpuinfo_init(const struct pqos_cpuinfo **topology);

/**
 * @brief Shuts down CPU information module
 *
 * @return Operation status
 * @retval 0 success
 * @retval -EPERM cpuinfo not initialized
 */
int cpuinfo_fini(void);

#ifdef __cplusplus
}
#endif

#endif /* __PQOS_CPUINFO_H__ */
