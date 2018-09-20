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
 * Monitoring module
 */

#include <stdint.h>
#include <stdio.h>
#include "pqos.h"

#ifndef __MONITOR_H__
#define __MONITOR_H__

#ifdef __cplusplus
extern "C" {
#endif

/**
 * @brief Verifies and translates multiple monitoring config strings into
 *        internal PID monitoring configuration
 *
 * @param arg argument passed to -p command line option
 */
void selfn_monitor_pids(const char *arg);

/**
 * @brief Looks for processes with highest CPU usage on the system and
 *        starts monitoring for them. Processes are displayed and sorted
 *        afterwards by LLC occupancy
 */
void selfn_monitor_top_pids(void);

/**
 * @brief Selects top-like monitoring format
 *
 * @param arg not used
 */
void selfn_monitor_top_like(const char *arg);

/**
 * @brief Selects monitoring interval
 *
 * @param arg string passed to -i command line option
 */
void selfn_monitor_interval(const char *arg);

/**
 * @brief Selects monitoring time
 *
 * @param arg string passed to -t command line option
 */
void selfn_monitor_time(const char *arg);

/**
 * @brief Selects type of monitoring output file
 *
 * @param arg string passed to -u command line option
 */
void selfn_monitor_file_type(const char *arg);

/**
 * @brief Selects monitoring output file
 *
 * @param arg string passed to -o command line option
 */
void selfn_monitor_file(const char *arg);

/**
 * @brief Translates multiple monitoring request strings into
 *        internal monitoring request structures
 *
 * @param str string passed to -m command line option
 */
void selfn_monitor_cores(const char *arg);

/**
 * @brief Stops monitoring on selected core(s)/pid(s)
 *
 */
void monitor_stop(void);

/**
 * @brief Starts monitoring on selected core(s)/pid(s)
 *
 * @param [in] cpu_info cpu information structure
 * @param [in] cap_mon monitoring capability
 *
 * @return Operation status
 * @retval 0 OK
 * @retval -1 error
 */
int monitor_setup(const struct pqos_cpuinfo *cpu_info,
                  const struct pqos_capability * const cap_mon);

/**
 * @brief Frees any allocated memory during parameter selection and
 *        monitoring setup.
 */
void monitor_cleanup(void);

/**
 * @brief Monitors resources and writes data into selected stream.
 */
void monitor_loop(void);

#ifdef __cplusplus
}
#endif

#endif /* __MONITOR_H__ */
