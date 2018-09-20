/*
 * BSD LICENSE
 *
 * Copyright(c) 2017 Intel Corporation. All rights reserved.
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
 * @brief Internal header file to PQoS OS monitoring module
 */
#ifndef __PQOS_OS_MON_H__
#define __PQOS_OS_MON_H__

#ifdef __cplusplus
extern "C" {
#endif

/**
 * @brief Initializes Perf structures used for OS monitoring interface
 *
 * @param cpu cpu topology structure
 * @param cap capabilities structure
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK success
 */
int os_mon_init(const struct pqos_cpuinfo *cpu, const struct pqos_cap *cap);

/**
 * @brief Shuts down monitoring sub-module for OS monitoring
 *
 * @return Operation status
 * @retval PQOS_RETVAL_OK success
 */
int os_mon_fini(void);

/*
 * @brief This function stops all perf counters
 *
 * Stops all counters and frees associated data structures
 *
 * @param group monitoring structure
 *
 * @return Operation status
 * @retval PQOS_RETVAL_OK on success
 * @retval PQOS_RETVAL_ERROR if error occurs
 */
int
os_mon_stop(struct pqos_mon_data *group);

/**
 * @brief OS interface to start resource monitoring on selected
 * group of cores
 *
 * The function sets up content of the \a group structure.
 *
 * Note that \a event cannot select PQOS_PERF_EVENT_IPC or
 * PQOS_PERF_EVENT_L3_MISS events without any PQoS event
 * selected at the same time.
 *
 * @param [in] num_cores number of cores in \a cores array
 * @param [in] cores array of logical core id's
 * @param [in] event combination of monitoring events
 * @param [in] context a pointer for application's convenience
 *            (unused by the library)
 * @param [in,out] group a pointer to monitoring structure
 *
 * @return Operations status
 * @retval PQOS_RETVAL_OK on success
 */
int os_mon_start(const unsigned num_cores,
                 const unsigned *cores,
                 const enum pqos_mon_event event,
                 void *context,
                 struct pqos_mon_data *group);

/**
 * @brief OS interface to poll monitoring data from requested groups
 *
 * @param [in] groups table of monitoring group pointers to be be updated
 * @param [in] num_groups number of monitoring groups in the table
 *
 * @return Operations status
 * @retval PQOS_RETVAL_OK on success
 */
int
os_mon_poll(struct pqos_mon_data **groups,
            const unsigned num_groups);

/**
 * @brief This function starts all perf counters for a task
 *
 * @param group monitoring structure
 *
 * @return Operation status
 * @retval PQOS_RETVAL_OK on success
 */
int
os_mon_start_pid(struct pqos_mon_data *group);

#ifdef __cplusplus
}
#endif

#endif /* __PQOS_OS_MON_H__ */
