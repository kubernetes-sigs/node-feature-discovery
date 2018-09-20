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

/**
 * @brief Data pseudo locking module
 */

#ifndef __DLOCK_H__
#define __DLOCK_H__

#ifdef __cplusplus
extern "C" {
#endif

/**
 * @brief Initializes data pseudo lock module
 *
 * @note It is assumed that \a clos is not associated to any CPU.
 * @note It is assumed PQoS library is already initialized.
 * @note Function modifies CAT classes on all sockets.
 *       This configuration is restored at dlock_exit().
 * @note Data will be locked in ways corresponding to least significant bits of
 *       the bit mask.
 * @note It is not allowed to initialize the module multiple times for
 *       different memory blocks.
 *
 * @param ptr pointer to memory block to be locked.
 *            If NULL then memory block is allocated.
 * @param size size of memory block to be locked
 * @param clos CAT class of service to be used for data locking
 * @param cpuid CPU ID to be used for data locking
 *
 * @return Operation status
 * @retval 0 OK
 * @retval <0 error
 */
int dlock_init(void *ptr, const size_t size, const int clos, const int cpuid);

/**
 * @brief Shuts down data pseudo lock module
 *
 * @note CAT configuration modified at dlock_init() is restored here.
 *
 * @return Operation status
 * @retval 0 OK
 * @retval <0 error
 */
int dlock_exit(void);

#ifdef __cplusplus
}
#endif

#endif /* __DLOCK_H__ */
