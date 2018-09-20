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
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.O
 *
 */

/**
 * @brief Provides access to machine operations (CPUID, MSR read & write)
 */

#ifndef __PQOS_MACHINE_H__
#define __PQOS_MACHINE_H__

#include <stdint.h>
#include <stdlib.h>
#include "types.h"

#ifdef __cplusplus
extern "C" {
#endif

#define MACHINE_DEFAULT_MAX_COREID  255       /**< max core id */

#define MACHINE_RETVAL_OK           0         /**< everything OK */
#define MACHINE_RETVAL_ERROR        1         /**< generic error */
#define MACHINE_RETVAL_PARAM        2         /**< parameter error */

/**
 * Results of CPUID operation are stored in this structure.
 * It consists of 4x32bits IA registers: EAX, EBX, ECX and EDX.
 */
struct cpuid_out {
        uint32_t eax;
        uint32_t ebx;
        uint32_t ecx;
        uint32_t edx;
};

/**
 * @brief Initializes machine module
 *
 * @param [in] max_core_id maximum logical core id to be handled by machine
 *             module. If zero then default value assumed
 *             \a MACHINE_DEFAULT_MAX_COREID
 *
 * @return Operation status
 * @retval MACHINE_RETVAL_OK on success
 */
int machine_init(const unsigned max_core_id);

/**
 * @brief Shuts down machine module
 *
 * @return Operation status
 * @retval MACHINE_RETVAL_OK on success
 */
int machine_fini(void);

/**
 * @brief Executes CPUID.leaf.sbuleaf on current core
 *
 * @param [in] leaf CPUID leaf number
 * @param [in] subleaf CPUID sub-leaf number
 * @param [out] out structure to write CPUID results into
 */
void
lcpuid(const unsigned leaf,
       const unsigned subleaf,
       struct cpuid_out *out);

/**
 * @brief Executes RDMSR on \a lcore logical core
 *
 * @param [in] lcore logical core id
 * @param [in] reg MSR to read from
 * @param [out] value place to store MSR value at
 *
 * @return Operation status
 * @retval MACHINE_RETVAL_OK on success
 */
int
msr_read(const unsigned lcore,
         const uint32_t reg,
         uint64_t *value);

/**
 * @brief Executes WRMSR on \a lcore logical core
 *
 * @param [in] lcore logical core id
 * @param [in] reg MSR to write to
 * @param [in] value to be written into \a reg
 *
 * @return Operation status
 * @retval MACHINE_RETVAL_OK on success
 */
int
msr_write(const unsigned lcore,
          const uint32_t reg,
          const uint64_t value);

#ifdef __cplusplus
}
#endif

#endif /* __PQOS_MACHINE_H__ */
