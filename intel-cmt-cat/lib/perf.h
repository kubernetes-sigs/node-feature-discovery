/*
 * BSD LICENSE
 *
 * Copyright(c) 2015 Intel Corporation. All rights reserved.
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

#ifndef __PQOS_PERF_H__
#define __PQOS_PERF_H__

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <unistd.h>
#include <linux/perf_event.h>

/**
 * @brief Function to setup perf event counters
 *
 * @param attr perf event attribute structure
 * @param pid pid to monitor
 * @param cpu cpu to monitor
 * @param group_fd fd of group leader (-1 for no leader)
 * @param flags perf event flags
 * @param counter_fd pointer to counter fd variable to be set
 *
 * @return fd used to read specified perf event counter
 * @retval positive number on success
 * @retval negative number on error
 */
int
perf_setup_counter(struct perf_event_attr *attr,
                   const pid_t pid,
                   const int cpu,
                   const int group_fd,
                   const unsigned long flags,
                   int *counter_fd);

/**
 * @brief Function to shutdown a perf event counter
 *
 * @param counter_fd fd used to access the perf counters
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on sucess
 */
int
perf_shutdown_counter(int counter_fd);

/**
 * @brief Function to start a perf counter
 *
 * @param counter_fd fd used to access the perf counter
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on sucess
 */
int
perf_start_counter(int counter_fd);

/**
 * @brief Function to stop a perf counter
 *
 * @param counter_fd fd used to access the perf counter
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on sucess
 */
int
perf_stop_counter(int counter_fd);

/**
 * @brief Function to read a perf counter
 *
 * @param counter_fd fd used to access the perf counter
 * @param value pointer to variable to store counter value
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on sucess
 */
int
perf_read_counter(int counter_fd, uint64_t *value);

#ifdef __cplusplus
}
#endif
#endif
