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

%module pqos

%include typemaps.i
int pqos_alloc_assoc_get(const unsigned, unsigned *OUTPUT);
unsigned *pqos_cpu_get_sockets(const struct pqos_cpuinfo *, unsigned *OUTPUT);
unsigned *pqos_cpu_get_l2ids(const struct pqos_cpuinfo *, unsigned *OUTPUT);
int pqos_cpu_get_socketid(const struct pqos_cpuinfo *, const unsigned, unsigned *OUTPUT);
int pqos_mon_assoc_get(const unsigned lcore, pqos_rmid_t *OUTPUT);

%{
/* Includes the header in the wrapper code */
#include <pqos.h>

/**
 * @brief Helper function to get L2CA capabilities data
 *
 * @return Pointer to capabilities data or NULL on error
 */
struct pqos_cap_l2ca * get_cap_l2ca(void)
{
	const struct pqos_cap *cap = NULL;
	const struct pqos_capability *cap_l2ca = NULL;
	int ret = 0;

	/* Get capability pointer */
	ret = pqos_cap_get(&cap, NULL);
	if (ret != PQOS_RETVAL_OK || cap == NULL) {
		printf("PQOS: Error retrieving PQoS capabilities!\n");
		return NULL;
	}

	/* Get L2CA capabilities */
	ret = pqos_cap_get_type(cap, PQOS_CAP_TYPE_L2CA, &cap_l2ca);
	if (ret != PQOS_RETVAL_OK || cap_l2ca == NULL) {
		printf("PQOS: Error retrieving PQOS_CAP_TYPE_L2CA "
			"capabilities!\n");
		return NULL;
	}

	return cap_l2ca->u.l2ca;
}

/**
 * @brief Helper function to get L3CA capabilities data
 *
 * @return Pointer to capabilities data or NULL on error
 */
struct pqos_cap_l3ca * get_cap_l3ca(void)
{
	const struct pqos_cap *cap = NULL;
	const struct pqos_capability *cap_l3ca = NULL;
	int ret = 0;

	/* Get capability pointer */
	ret = pqos_cap_get(&cap, NULL);
	if (ret != PQOS_RETVAL_OK || cap == NULL) {
		printf("PQOS: Error retrieving PQoS capabilities!\n");
		return NULL;
	}

	/* Get L3CA capabilities */
	ret = pqos_cap_get_type(cap, PQOS_CAP_TYPE_L3CA, &cap_l3ca);
	if (ret != PQOS_RETVAL_OK || cap_l3ca == NULL) {
		printf("PQOS: Error retrieving PQOS_CAP_TYPE_L3CA "
			"capabilities!\n");
		return NULL;
	}

	return cap_l3ca->u.l3ca;
}

/**
 * @brief Helper function to get CPUINFO data
 *
 * @return Pointer to data or NULL on error
 */
const struct pqos_cpuinfo * get_cpuinfo(void)
{
	const struct pqos_cpuinfo *cpu = NULL;
	int ret = 0;

	/* Get CPU info pointer */
	ret = pqos_cap_get(NULL, &cpu);
	if (ret != PQOS_RETVAL_OK || cpu == NULL) {
		printf("PQOS: Error retrieving cpu info!\n");
		return NULL;
	}

	return cpu;
}

/**
 * @brief Helper function to get single L2 COS configuration
 *
 * @param [in] l2_id - L2 cache id
 * @param [in] cos_id - id of L2CA class of service to read
 * @param [out] l2ca - read class of service
 *
 * @return Operation status (0 on success)
 */
int get_l2ca(struct pqos_l2ca *l2ca, unsigned int l2_id, unsigned int cos_id)
{
	unsigned num = 0;
	unsigned i = 0;
	int ret = -1;
	struct pqos_l2ca *temp_ca = NULL;
	struct pqos_cap_l2ca *cap_l2ca = NULL;

	cap_l2ca = get_cap_l2ca();
	if (cap_l2ca == NULL)
		return -1;

	const unsigned max_cos_num = cap_l2ca->num_classes;
	temp_ca = (struct pqos_l2ca *) malloc(sizeof(*temp_ca) * max_cos_num);
	if (temp_ca == NULL)
		return -1;

	memset(temp_ca, 0, sizeof(*temp_ca) * max_cos_num);

	if (pqos_l2ca_get(l2_id, max_cos_num, &num, temp_ca) !=
			PQOS_RETVAL_OK) {
		printf("PQOS: Error retrieving L2 COS!\n");
		goto exit;
	}

	for (i = 0; i < num;  i++) {
		if (temp_ca[i].class_id == cos_id) {
			*l2ca = temp_ca[i];
			ret = 0;
			break;
		}
	}

	if (ret == -1)
		printf("PQOS: Error retrieving L2 COS! COS not found!\n");

exit:
	if (temp_ca != NULL)
		free(temp_ca);

	return ret;
}

/**
 * @brief Helper function to get single L3 COS configuration
 *
 * @param [in] socket - CPU socket id
 * @param [in] cos_id - id of L3CA class of service to read
 * @param [out] l3ca - read class of service
 *
 * @return Operation status (0 on success)
 */
int get_l3ca(struct pqos_l3ca *l3ca, unsigned int socket, unsigned int cos_id)
{
	unsigned num = 0;
	unsigned i = 0;
	int ret = -1;
	struct pqos_l3ca *temp_ca = NULL;
	struct pqos_cap_l3ca *cap_l3ca = NULL;

	cap_l3ca = get_cap_l3ca();
	if (cap_l3ca == NULL)
		return -1;

	const unsigned max_cos_num = cap_l3ca->num_classes;
	temp_ca = (struct pqos_l3ca *) malloc(sizeof(*temp_ca) * max_cos_num);
	if (temp_ca == NULL)
		return -1;

	memset(temp_ca, 0, sizeof(*temp_ca) * max_cos_num);

	if (pqos_l3ca_get(socket, max_cos_num, &num, temp_ca) !=
			PQOS_RETVAL_OK) {
		printf("PQOS: Error retrieving L3 COS!\n");
		goto exit;
	}

	for (i = 0; i < num;  i++) {
		if (temp_ca[i].class_id == cos_id) {
			*l3ca = temp_ca[i];
			ret = 0;
			break;
		}
	}

	if (ret == -1)
		printf("PQOS: Error retrieving L3 COS! COS not found!\n");

exit:
	if (temp_ca != NULL)
		free(temp_ca);

	return ret;
}
%}

/* Parse the header file to generate wrappers */
%include <pqos.h>

%include carrays.i
%include cpointer.i
%include stdint.i

/* Generate wrappers around C arrays */
%array_functions(unsigned int, uint_a);
%array_functions(struct pqos_mon_data*, pqos_mon_data_p_a);

/* Generate wrappers around C pointers */
%pointer_functions(int, intp);
%pointer_functions(unsigned int, uintp);
%pointer_functions(struct pqos_cap_l2ca, l2ca_cap_p);
%pointer_functions(struct pqos_cap_l3ca, l3ca_cap_p);
%pointer_functions(struct pqos_mon_data, pqos_mon_data_p);
%pointer_functions(struct pqos_monitor*, pqos_monitor_p_p);
%pointer_functions(struct pqos_cap*, pqos_cap_p_p);
%pointer_functions(struct pqos_cpuinfo, cpuinfo_p);
%pointer_functions(struct pqos_cpuinfo*, pqos_cpuinfo_p_p);

/* Helper functions for libpqos */
struct pqos_cap_l2ca * get_cap_l2ca(void);
struct pqos_cap_l3ca * get_cap_l3ca(void);
const struct pqos_cpuinfo * get_cpuinfo(void);
int get_l2ca(struct pqos_l2ca *l2ca, unsigned int socket, unsigned int idx);
int get_l3ca(struct pqos_l3ca *l3ca, unsigned int socket, unsigned int idx);
