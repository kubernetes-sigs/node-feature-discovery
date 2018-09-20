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
 * @brief Internal header file to share PQoS API lock mechanism
 * and library initialization status.
 */

#ifndef __PQOS_HOSTCAP_H__
#define __PQOS_HOSTCAP_H__

#ifdef __cplusplus
extern "C" {
#endif

/**
 * @brief Modifies L3 CAT capability structure upon CDP config change
 *
 * Limited error checks done in this function and no errors reported.
 * It is up to caller to check for L3 CAT & CDP support.
 *
 * @param [in] prev old CDP setting
 * @param [in] next new CDP setting
 */
void _pqos_cap_l3cdp_change(const int prev, const int next);

/**
 * @brief Aquires lock for PQoS API use
 *
 * Only one thread at a time is allowed to use the API.
 * Each PQoS API need to use api_lock and api_unlock functions.
 */
void _pqos_api_lock(void);

/**
 * @brief Symmetric operation to \a _pqos_api_lock to release the lock
 */
void _pqos_api_unlock(void);

/**
 * @brief Checks library initialization state
 *
 * @param expect expected stated of library initialization state
 *
 * @return Check status
 * @retval PQOS_RETVAL_OK state as expected
 * @retval PQOS_RETVA_ERROR state different than expected
 */
int _pqos_check_init(const int expect);

#ifdef __cplusplus
}
#endif

#endif /* __PQOS_HOSTCAP_H__ */
