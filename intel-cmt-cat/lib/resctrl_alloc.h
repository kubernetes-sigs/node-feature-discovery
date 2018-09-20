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
 * @brief Internal header file for resctrl allocation
 */

#ifndef __PQOS_RESCTRL_ALLOC_H__
#define __PQOS_RESCTRL_ALLOC_H__

#include <limits.h>                     /**< CHAR_BIT*/

#include "pqos.h"

#ifdef __cplusplus
extern "C" {
#endif

#ifndef RESCTRL_ALLOC_PATH
#define RESCTRL_ALLOC_PATH "/sys/fs/resctrl"
#endif

/**
 * Max supported number of CPU's
 */
#define RESCTRL_ALLOC_MAX_CPUS 4096

/**
 * @brief Retrieves number of resctrl groups allowed
 *
 * @param [in] cap platform QoS capabilities structure
 *                 returned by \a pqos_cap_get
 * @param grps_num place to store number of groups
 *
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_get_grps_num(const struct pqos_cap *cap, unsigned *grps_num);

/**
 * @brief Structure to hold parsed cpu mask
 *
 * Structure contains table with cpu bit mask. Each table item holds
 * information about 8 bit in mask.
 *
 * Example bitmask tables:
 *  - cpus file contains 'ABC' mask = [ ..., 0x0A, 0xBC ]
 *  - cpus file contains 'ABCD' mask = [ ..., 0xAB, 0xCD ]
 */
struct resctrl_alloc_cpumask {
	uint8_t tab[RESCTRL_ALLOC_MAX_CPUS / CHAR_BIT];  /**< bit mask table */
};

/**
 * @brief Set lcore bit in cpu mask
 *
 * @param [in] lcore Core number
 * @param [in] cpumask Modified cpu mask
 */
void resctrl_alloc_cpumask_set(const unsigned lcore,
                               struct resctrl_alloc_cpumask *mask);

/**
 * @brief Check if lcore is set in cpu mask
 *
 * @param [in] lcore Core number
 * @param [in] cpumask Cpu mask
 *
 * @return Returns 1 when bit corresponding to lcore is set in mask
 * @retval 1 if cpu bit is set in mask
 * @retval 0 if cpu bit is not set in mask
 */
int resctrl_alloc_cpumask_get(const unsigned lcore,
	                      const struct resctrl_alloc_cpumask *mask);

/**
 * @brief Write CPU mask to file
 *
 * @param [in] class_id COS id
 * @param [in] mask CPU mask to write
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_cpumask_write(const unsigned class_id,
                                const struct resctrl_alloc_cpumask *mask);

/**
 * @brief Read CPU mask from file
 *
 * @param [in] class_id COS id
 * @param [out] mask CPU mask to write
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_cpumask_read(const unsigned class_id,
	                       struct resctrl_alloc_cpumask *mask);

/*
 * @brief Structure to hold parsed schemata
 */
struct resctrl_alloc_schemata {
	unsigned l3ca_num;      /**< Number of L3 COS held in struct */
	struct pqos_l3ca *l3ca; /**< L3 COS definitions */
	unsigned l2ca_num;      /**< Number of L2 COS held in struct */
	struct pqos_l2ca *l2ca; /**< L2 COS definitions */
	unsigned mba_num;       /**< Number of MBA COS held in struct */
	struct pqos_mba *mba;   /**< MBA COS definitions */
};

/*
 * @brief Deallocate memory of schemata struct
 *
 * @param[in] schemata Schemata structure
 */
void resctrl_alloc_schemata_fini(struct resctrl_alloc_schemata *schemata);

/**
 * @brief Allocates memory of schemata struct
 *
 * @param [in] class_id COS id
 * @param [in] cap platform QoS capabilities structure
 *                 returned by \a pqos_cap_get
 * @param [in] cpu CPU information structure from \a pqos_cap_get
 * @param[out] schemata Schemata structure
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_schemata_init(const unsigned class_id,
	                        const struct pqos_cap *cap,
	                        const struct pqos_cpuinfo *cpu,
	                        struct resctrl_alloc_schemata *schemata);

/**
 * @brief Read resctrl schemata from file
 *
 * @param [in] class_is COS id
 * @param [out] schemata Parsed schemata
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_schemata_read(const unsigned class_id,
	                        struct resctrl_alloc_schemata *schemata);

/**
 * @brief Write resctrl schemata to file
 *
 * @param [in] class_id COS id
 * @param [in] schemata Schemata to write
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_schemata_write(const unsigned class_id,
	                         const struct resctrl_alloc_schemata *schemata);

/**
 * @brief Function to validate if \a task is a valid task ID
 *
 * @param task task ID to validate
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_task_validate(const pid_t task);

/**
 * @brief Function to write task ID to resctrl COS tasks file
 *        Used to associate a task with COS
 *
 * @param class_id COS tasks file to write to
 * @param task task ID to write to tasks file
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_task_write(const unsigned class_id, const pid_t task);

/**
 * @brief Reads task id's from resctrl task file for a given COS
 *
 * @param [in] class_id Class of Service ID
 * @param [out] count place to store actual number of task id's returned
 *
 * @return Allocated task id array
 * @retval NULL on error
 */
unsigned *resctrl_alloc_task_read(unsigned class_id, unsigned *count);

/**
 * @brief Function to search a COS tasks file for a task ID
 *
 * @param [out] class_id COS containing task ID
 * @param [in] cap platform QoS capabilities structure
 *                 returned by \a pqos_cap_get
 * @param [in] task task ID to search for
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
int resctrl_alloc_task_search(unsigned *class_id,
                              const struct pqos_cap *cap,
                              const pid_t task);

/**
 * @brief Function to search a COS tasks file and check if this file is blank
 *
 * @param [in] class_id COS containing task ID
 * @param [out] found flag
 *                    0 if no Task ID is found
 *                    1 if a Task ID is found
 *
 * @return Operation status
 */
int resctrl_alloc_task_file_check(const unsigned class_id, unsigned *found);

#ifdef __cplusplus
}
#endif

#endif /* __PQOS_RESCTRL_ALLOC_H__ */

