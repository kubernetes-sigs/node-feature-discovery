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
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <errno.h>

#include "log.h"
#include "types.h"
#include "resctrl_alloc.h"

/*
 * COS file names on resctrl file system
 */
static const char *rctl_cpus = "cpus";
static const char *rctl_schemata = "schemata";
static const char *rctl_tasks = "tasks";

int
resctrl_alloc_get_grps_num(const struct pqos_cap *cap, unsigned *grps_num)
{
	unsigned i;
	unsigned max_rctl_grps = 0;
	int ret = PQOS_RETVAL_OK;

	ASSERT(cap != NULL);
	ASSERT(grps_num != NULL);

	/*
	 * Loop through all caps that have OS support
	 * Find max COS supported by all
	 */
	for (i = 0; i < cap->num_cap; i++) {
		unsigned num_cos = 0;
		const struct pqos_capability *p_cap = &cap->capabilities[i];

		if (!p_cap->os_support)
			continue;

		/* get L3 CAT COS num */
		if (p_cap->type == PQOS_CAP_TYPE_L3CA) {
			ret = pqos_l3ca_get_cos_num(cap, &num_cos);
			if (ret != PQOS_RETVAL_OK)
				return ret;

			if (max_rctl_grps == 0)
				max_rctl_grps = num_cos;
			else if (num_cos < max_rctl_grps)
				max_rctl_grps = num_cos;
		}
		/* get L2 CAT COS num */
		if (p_cap->type == PQOS_CAP_TYPE_L2CA) {
			ret = pqos_l2ca_get_cos_num(cap, &num_cos);
			if (ret != PQOS_RETVAL_OK)
				return ret;

			if (max_rctl_grps == 0)
				max_rctl_grps = num_cos;
			else if (num_cos < max_rctl_grps)
				max_rctl_grps = num_cos;
		}
		/* get MBA COS num */
		if (p_cap->type == PQOS_CAP_TYPE_MBA) {
			ret = pqos_mba_get_cos_num(cap, &num_cos);
			if (ret != PQOS_RETVAL_OK)
				return ret;

			if (max_rctl_grps == 0)
				max_rctl_grps = num_cos;
			else if (num_cos < max_rctl_grps)
				max_rctl_grps = num_cos;
		}
	}
	*grps_num = max_rctl_grps;
	return PQOS_RETVAL_OK;
}

/**
 * @brief Converts string into 64-bit unsigned number.
 *
 * Numbers can be in decimal or hexadecimal format.
 *
 * @param [in] s string to be converted into 64-bit unsigned number
 * @param [in] base Numerical base
 * @param [out] Numeric value of the string representing the number
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
static int
strtouint64(const char *s, int base, uint64_t *value)
{
	char *endptr = NULL;

	ASSERT(s != NULL);
	if (strncasecmp(s, "0x", 2) == 0) {
		base = 16;
		s += 2;
	}

	*value = strtoull(s, &endptr, base);
	if (!(*s != '\0' && (*endptr == '\0' || *endptr == '\n')))
		return PQOS_RETVAL_ERROR;

	return PQOS_RETVAL_OK;
}

/**
 * @brief Opens COS file in resctl filesystem
 *
 * @param [in] class_id COS id
 * @param [in] name File name
 * @param [in] mode fopen mode
 *
 * @return Pointer to the stream
 * @retval Pointer on success
 * @retval NULL on error
 */
static FILE *
resctrl_alloc_fopen(const unsigned class_id, const char *name, const char *mode)
{
	FILE *fd;
	char buf[128];
	int result;

	ASSERT(name != NULL);
	ASSERT(mode != NULL);

	memset(buf, 0, sizeof(buf));
	if (class_id == 0)
		result = snprintf(buf, sizeof(buf) - 1,
				  "%s/%s", RESCTRL_ALLOC_PATH, name);
	else
		result = snprintf(buf, sizeof(buf) - 1, "%s/COS%u/%s",
				  RESCTRL_ALLOC_PATH, class_id, name);

	if (result < 0)
		return NULL;

	fd = fopen(buf, mode);
	if (fd == NULL)
		LOG_ERROR("Could not open %s file %s for COS %u\n",
			  name, buf, class_id);

	return fd;
}

/**
 * @brief Closes COS file in resctl filesystem
 *
 * @param[in] fd File descriptor
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
static int
resctrl_alloc_fclose(FILE *fd)
{
	if (fd == NULL)
		return PQOS_RETVAL_PARAM;

	if (fclose(fd) == 0)
		return PQOS_RETVAL_OK;

	switch (errno) {
	case EBADF:
		LOG_ERROR("Invalid file descriptor!\n");
		break;
	case EINVAL:
		LOG_ERROR("Invalid file arguments!\n");
		break;
	default:
		LOG_ERROR("Error closing file!\n");
	}

	return PQOS_RETVAL_ERROR;
}

/*
 * ---------------------------------------
 * CPU mask structures and utility functions
 * ---------------------------------------
 */

void
resctrl_alloc_cpumask_set(const unsigned lcore,
                          struct resctrl_alloc_cpumask *mask)
{
	/* index in mask table */
	const unsigned item = (sizeof(mask->tab) - 1) - (lcore / CHAR_BIT);
	const unsigned bit = lcore % CHAR_BIT;

	/* Set lcore bit in mask table item */
	mask->tab[item] = mask->tab[item] | (1 << bit);
}

int
resctrl_alloc_cpumask_get(const unsigned lcore,
                          const struct resctrl_alloc_cpumask *mask)
{
	/* index in mask table */
	const unsigned item = (sizeof(mask->tab) - 1) - (lcore / CHAR_BIT);
	const unsigned bit = lcore % CHAR_BIT;

	/* Check if lcore bit is set in mask table item */
	return (mask->tab[item] >> bit) & 0x1;
}

int
resctrl_alloc_cpumask_write(const unsigned class_id,
	                    const struct resctrl_alloc_cpumask *mask)
{
	int ret = PQOS_RETVAL_OK;
	FILE *fd;
	unsigned  i;

	fd = resctrl_alloc_fopen(class_id, rctl_cpus, "w");
	if (fd == NULL)
		return PQOS_RETVAL_ERROR;

	for (i = 0; i < sizeof(mask->tab); i++) {
		const unsigned value = (unsigned) mask->tab[i];

		if (fprintf(fd, "%02x", value) < 0) {
			LOG_ERROR("Failed to write cpu mask\n");
			break;
		}
		if ((i + 1) % 4 == 0)
			if (fprintf(fd, ",") < 0) {
				LOG_ERROR("Failed to write cpu mask\n");
				break;
			}
	}
	ret = resctrl_alloc_fclose(fd);

	/* check if error occured in loop */
	if (i < sizeof(mask->tab))
		return PQOS_RETVAL_ERROR;

	return ret;
}

int
resctrl_alloc_cpumask_read(const unsigned class_id,
			   struct resctrl_alloc_cpumask *mask)
{
	int i, hex_offset, idx;
	FILE *fd;
	size_t num_chars = 0;
	char cpus[RESCTRL_ALLOC_MAX_CPUS / CHAR_BIT];

	memset(mask, 0, sizeof(struct resctrl_alloc_cpumask));
	memset(cpus, 0, sizeof(cpus));
	fd = resctrl_alloc_fopen(class_id, rctl_cpus, "r");
	if (fd == NULL)
		return PQOS_RETVAL_ERROR;

	/** Read the entire file into memory. */
	num_chars = fread(cpus, sizeof(char), sizeof(cpus), fd);

	if (ferror(fd) != 0) {
		LOG_ERROR("Error reading CPU file\n");
		resctrl_alloc_fclose(fd);
		return PQOS_RETVAL_ERROR;
	}
	cpus[sizeof(cpus) - 1] = '\0'; /** Just to be safe. */
	if (resctrl_alloc_fclose(fd) != PQOS_RETVAL_OK)
		return PQOS_RETVAL_ERROR;

	/**
	 *  Convert the cpus array into hex, skip any non hex chars.
	 *  Store the hex values in the mask tab.
	 */
	for (i = num_chars - 1, hex_offset = 0, idx = sizeof(mask->tab) - 1;
	     i >= 0; i--) {
		const char c = cpus[i];
		int hex_num;

		if ('0' <= c && c <= '9')
			hex_num = c - '0';
		else if ('a' <= c && c <= 'f')
			hex_num = 10 + c - 'a';
		else if ('A' <= c && c <= 'F')
			hex_num = 10 + c - 'A';
		else
			continue;

		if (!hex_offset)
			mask->tab[idx] = (uint8_t) hex_num;
		else {
			mask->tab[idx] |= (uint8_t) (hex_num << 4);
			idx--;
		}
		hex_offset ^= 1;
	}

	return PQOS_RETVAL_OK;
}

/*
 * ---------------------------------------
 * Schemata structures and utility functions
 * ---------------------------------------
 */

void
resctrl_alloc_schemata_fini(struct resctrl_alloc_schemata *schemata)
{
	if (schemata->l2ca != NULL) {
		free(schemata->l2ca);
		schemata->l2ca = NULL;
	}
	if (schemata->l3ca != NULL) {
		free(schemata->l3ca);
		schemata->l3ca = NULL;
	}
	if (schemata->mba != NULL) {
		free(schemata->mba);
		schemata->mba = NULL;
	}
}

int
resctrl_alloc_schemata_init(const unsigned class_id,
			    const struct pqos_cap *cap,
			    const struct pqos_cpuinfo *cpu,
			    struct resctrl_alloc_schemata *schemata)
{
	int ret = PQOS_RETVAL_OK;
	int retval;
	unsigned num_cos, num_ids, i;

	ASSERT(schemata != NULL);

	memset(schemata, 0, sizeof(struct resctrl_alloc_schemata));

	/* L2 */
	retval = pqos_l2ca_get_cos_num(cap, &num_cos);
	if (retval == PQOS_RETVAL_OK && class_id < num_cos) {
		unsigned *l2ids = NULL;

		l2ids = pqos_cpu_get_l2ids(cpu, &num_ids);
		if (l2ids == NULL) {
			ret = PQOS_RETVAL_ERROR;
			goto resctrl_alloc_schemata_init_exit;
		}

		free(l2ids);

		schemata->l2ca_num = num_ids;
		schemata->l2ca = calloc(num_ids, sizeof(struct pqos_l2ca));
		if (schemata->l2ca == NULL) {
			ret = PQOS_RETVAL_ERROR;
			goto resctrl_alloc_schemata_init_exit;
		}

		/* fill class_id */
		for (i = 0; i < num_ids; i++)
			schemata->l2ca[i].class_id = class_id;
	}

	/* L3 */
	retval = pqos_l3ca_get_cos_num(cap, &num_cos);
	if (retval == PQOS_RETVAL_OK && class_id < num_cos) {
		unsigned *sockets = NULL;
		int cdp_enabled;

		sockets = pqos_cpu_get_sockets(cpu, &num_ids);
		if (sockets == NULL) {
			ret = PQOS_RETVAL_ERROR;
			goto resctrl_alloc_schemata_init_exit;
		}

		free(sockets);

		schemata->l3ca_num = num_ids;
		schemata->l3ca = calloc(num_ids, sizeof(struct pqos_l3ca));
		if (schemata->l3ca == NULL) {
			ret = PQOS_RETVAL_ERROR;
			goto resctrl_alloc_schemata_init_exit;
		}

		ret = pqos_l3ca_cdp_enabled(cap, NULL, &cdp_enabled);
		if (ret != PQOS_RETVAL_OK)
			goto resctrl_alloc_schemata_init_exit;

		/* fill class_id and cdp values */
		for (i = 0; i < num_ids; i++) {
			schemata->l3ca[i].class_id = class_id;
			schemata->l3ca[i].cdp = cdp_enabled;
		}
	}

	/* MBA */
	retval = pqos_mba_get_cos_num(cap, &num_cos);
	if (retval == PQOS_RETVAL_OK && class_id < num_cos) {
		unsigned *sockets = NULL;

		sockets = pqos_cpu_get_sockets(cpu, &num_ids);
		if (sockets == NULL) {
			ret = PQOS_RETVAL_ERROR;
			goto resctrl_alloc_schemata_init_exit;
		}

		free(sockets);

		schemata->mba_num = num_ids;
		schemata->mba = calloc(num_ids, sizeof(struct pqos_mba));
		if (schemata->mba == NULL) {
			ret = PQOS_RETVAL_ERROR;
			goto resctrl_alloc_schemata_init_exit;
		}

		/* fill class_id */
		for (i = 0; i < num_ids; i++) {
			schemata->mba[i].class_id = class_id;
			schemata->mba[i].mb_rate = 100;
		}
	}

 resctrl_alloc_schemata_init_exit:
	/* Deallocate memory in case of error */
	if (ret != PQOS_RETVAL_OK)
		resctrl_alloc_schemata_fini(schemata);

	return ret;
}

/**
 * @brief Schemata type
 */
enum resctrl_alloc_schemata_type {
	RESCTRL_ALLOC_SCHEMATA_TYPE_NONE,   /**< unknown */
	RESCTRL_ALLOC_SCHEMATA_TYPE_L2,     /**< L2 CAT */
	RESCTRL_ALLOC_SCHEMATA_TYPE_L3,     /**< L3 CAT without CDP */
	RESCTRL_ALLOC_SCHEMATA_TYPE_L3CODE, /**< L3 CAT code */
	RESCTRL_ALLOC_SCHEMATA_TYPE_L3DATA, /**< L3 CAT data */
	RESCTRL_ALLOC_SCHEMATA_TYPE_MB,     /**< MBA data */
};

/**
 * @brief Determine allocation type
 *
 * @param [in] str resctrl label
 *
 * @return Allocation type
 */
static int
resctrl_alloc_schemata_type_get(const char *str)
{
	int type = RESCTRL_ALLOC_SCHEMATA_TYPE_NONE;

	if (strcasecmp(str, "L2") == 0)
		type = RESCTRL_ALLOC_SCHEMATA_TYPE_L2;
	else if (strcasecmp(str, "L3") == 0)
		type = RESCTRL_ALLOC_SCHEMATA_TYPE_L3;
	else if (strcasecmp(str, "L3CODE") == 0)
		type = RESCTRL_ALLOC_SCHEMATA_TYPE_L3CODE;
	else if (strcasecmp(str, "L3DATA") == 0)
		type = RESCTRL_ALLOC_SCHEMATA_TYPE_L3DATA;
	else if (strcasecmp(str, "MB") == 0)
		type = RESCTRL_ALLOC_SCHEMATA_TYPE_MB;

	return type;
}

/**
 * @brief Fill schemata structure
 *
 * @param [in] res_id Resource id
 * @param [in] value Ways mask/Memory B/W rate
 * @param [in] type Schemata type
 * @param [out] schemata Schemata structure
 *
 * @return Operational status
 * @retval PQOS_RETVAL_OK on success
 */
static int
resctrl_alloc_schemata_set(const unsigned res_id,
	     const uint64_t value,
	     const int type,
	     struct resctrl_alloc_schemata *schemata)
{
	if (type == RESCTRL_ALLOC_SCHEMATA_TYPE_L2) {
		if (schemata->l2ca_num <= res_id)
			return PQOS_RETVAL_ERROR;
		schemata->l2ca[res_id].ways_mask = value;

	} else if (type == RESCTRL_ALLOC_SCHEMATA_TYPE_L3) {
		if (schemata->l3ca_num <= res_id || schemata->l3ca[res_id].cdp)
			return PQOS_RETVAL_ERROR;
		schemata->l3ca[res_id].u.ways_mask = value;

	} else if (type == RESCTRL_ALLOC_SCHEMATA_TYPE_L3CODE) {
		if (schemata->l3ca_num <= res_id || !schemata->l3ca[res_id].cdp)
			return PQOS_RETVAL_ERROR;
		schemata->l3ca[res_id].u.s.code_mask = value;

	} else if (type == RESCTRL_ALLOC_SCHEMATA_TYPE_L3DATA) {
		if (schemata->l3ca_num <= res_id || !schemata->l3ca[res_id].cdp)
			return PQOS_RETVAL_ERROR;
		schemata->l3ca[res_id].u.s.data_mask = value;

	} else if (type == RESCTRL_ALLOC_SCHEMATA_TYPE_MB) {
		if (schemata->mba_num <= res_id)
			return PQOS_RETVAL_ERROR;
		schemata->mba[res_id].mb_rate = value;
	}

	return PQOS_RETVAL_OK;
}

int
resctrl_alloc_schemata_read(const unsigned class_id,
			    struct resctrl_alloc_schemata *schemata)
{
	int ret = PQOS_RETVAL_OK;
	FILE *fd;
	int type = RESCTRL_ALLOC_SCHEMATA_TYPE_NONE;
	char buf[16 * 1024];
	char *p = NULL, *q = NULL, *saveptr = NULL;

	ASSERT(schemata != NULL);

	fd = resctrl_alloc_fopen(class_id, rctl_schemata, "r");
	if (fd == NULL)
		return PQOS_RETVAL_ERROR;

	if ((schemata->l3ca_num > 0 && schemata->l3ca == NULL)
	    || (schemata->l2ca_num > 0 && schemata->l2ca == NULL)) {
		ret = PQOS_RETVAL_ERROR;
		goto resctrl_alloc_schemata_read_exit;
	}

	memset(buf, 0, sizeof(buf));
	while (fgets(buf, sizeof(buf), fd) != NULL) {
		q = buf;
		/**
		 * Trim white spaces
		 */
		while (isspace(*q))
			q++;

		/**
		 * Determine allocation type
		 */
		p = strchr(q, ':');
		if (p == NULL) {
			ret = PQOS_RETVAL_ERROR;
			break;
		}
		*p = '\0';
		type = resctrl_alloc_schemata_type_get(q);

		/* Skip unknown label */
		if (type == RESCTRL_ALLOC_SCHEMATA_TYPE_NONE)
			continue;

		/**
		 * Parse COS masks
		 */
		for (++p; ; p = NULL) {
			char *token = NULL;
			uint64_t id = 0;
			uint64_t value = 0;
			unsigned base = (type == RESCTRL_ALLOC_SCHEMATA_TYPE_MB
				         ? 10 : 16);

			token = strtok_r(p, ";", &saveptr);
			if (token == NULL)
				break;

			q = strchr(token, '=');
			if (q == NULL) {
				ret = PQOS_RETVAL_ERROR;
				goto resctrl_alloc_schemata_read_exit;
			}
			*q = '\0';

			ret = strtouint64(token, 10, &id);
			if (ret != PQOS_RETVAL_OK)
				goto resctrl_alloc_schemata_read_exit;

			ret = strtouint64(q + 1, base, &value);
			if (ret != PQOS_RETVAL_OK)
				goto resctrl_alloc_schemata_read_exit;

			ret = resctrl_alloc_schemata_set(id,
				                         value,
				                         type,
				                         schemata);
			if (ret != PQOS_RETVAL_OK)
				goto resctrl_alloc_schemata_read_exit;
		}
	}

 resctrl_alloc_schemata_read_exit:
	/* check if error occured */
	if (ret != PQOS_RETVAL_OK)
		resctrl_alloc_fclose(fd);
	else
		ret = resctrl_alloc_fclose(fd);

	return ret;
}

int
resctrl_alloc_schemata_write(const unsigned class_id,
                             const struct resctrl_alloc_schemata *schemata)
{
	int ret = PQOS_RETVAL_OK;
	unsigned i;
	FILE *fd;
	char buf[16 * 1024];

	ASSERT(schemata != NULL);

	fd = resctrl_alloc_fopen(class_id, rctl_schemata, "w");
	if (fd == NULL)
		return PQOS_RETVAL_ERROR;

	/* Enable fully buffered output. File won't be flushed until 16kB
	 * buffer is full */
	if (setvbuf(fd, buf, _IOFBF, sizeof(buf)) != 0) {
		resctrl_alloc_fclose(fd);
		return PQOS_RETVAL_ERROR;
	}

	/* L2 */
	if (schemata->l2ca_num > 0) {
		fprintf(fd, "L2:");
		for (i = 0; i < schemata->l2ca_num; i++) {
			if (i > 0)
				fprintf(fd, ";");
			fprintf(fd, "%u=%x", i, schemata->l2ca[i].ways_mask);
		}
		fprintf(fd, "\n");
	}

	/* L3 without CDP */
	if (schemata->l3ca_num > 0 && !schemata->l3ca[0].cdp) {
		fprintf(fd, "L3:");
		for (i = 0; i < schemata->l3ca_num; i++) {
			if (i > 0)
				fprintf(fd, ";");
			fprintf(fd, "%u=%llx", i, (unsigned long long)
				schemata->l3ca[i].u.ways_mask);
		}
		fprintf(fd, "\n");
	}

	/* L3 with CDP */
	if (schemata->l3ca_num > 0 && schemata->l3ca[0].cdp) {
		fprintf(fd, "L3CODE:");
		for (i = 0; i < schemata->l3ca_num; i++) {
			if (i > 0)
				fprintf(fd, ";");
			fprintf(fd, "%u=%llx", i, (unsigned long long)
				schemata->l3ca[i].u.s.code_mask);
		}
		fprintf(fd, "\nL3DATA:");
		for (i = 0; i < schemata->l3ca_num; i++) {
			if (i > 0)
				fprintf(fd, ";");
			fprintf(fd, "%u=%llx", i, (unsigned long long)
				schemata->l3ca[i].u.s.data_mask);
		}
		fprintf(fd, "\n");
	}

	/* MBA */
	if (schemata->mba_num > 0) {
		fprintf(fd, "MB:");
		for (i = 0; i < schemata->mba_num; i++) {
			if (i > 0)
				fprintf(fd, ";");
			fprintf(fd, "%u=%u", i, schemata->mba[i].mb_rate);
		}
		fprintf(fd, "\n");
	}

	ret = resctrl_alloc_fclose(fd);

	return ret;
}

/**
 * ---------------------------------------
 * Task utility functions
 * ---------------------------------------
 */

int
resctrl_alloc_task_validate(const pid_t task)
{
	char buf[128];

	memset(buf, 0, sizeof(buf));
	snprintf(buf, sizeof(buf)-1, "/proc/%d", (int)task);
	if (access(buf, F_OK) != 0) {
		LOG_ERROR("Task %d does not exist!\n", (int)task);
		return PQOS_RETVAL_ERROR;
	}

	return PQOS_RETVAL_OK;
}

int
resctrl_alloc_task_write(const unsigned class_id, const pid_t task)
{
	FILE *fd;
	int ret;

	/* Check if task exists */
	ret = resctrl_alloc_task_validate(task);
	if (ret != PQOS_RETVAL_OK)
		return PQOS_RETVAL_PARAM;

	/* Open resctrl tasks file */
	fd = resctrl_alloc_fopen(class_id, rctl_tasks, "w");
	if (fd == NULL)
		return PQOS_RETVAL_ERROR;

	/* Write task ID to file */
	if (fprintf(fd, "%d\n", task) < 0) {
		LOG_ERROR("Failed to write to task %d to file!\n", (int) task);
		resctrl_alloc_fclose(fd);
		return PQOS_RETVAL_ERROR;
	}
	ret = resctrl_alloc_fclose(fd);

	return ret;
}

unsigned *
resctrl_alloc_task_read(unsigned class_id, unsigned *count)
{
	FILE *fd;
	unsigned *tasks = NULL, idx = 0;
	int ret;
	char buf[128];
	struct linked_list {
		uint64_t task_id;
		struct linked_list *next;
	} head, *current = NULL;

	/* Open resctrl tasks file */
	fd = resctrl_alloc_fopen(class_id, rctl_tasks, "r");
	if (fd == NULL)
		return NULL;

	head.next = NULL;
	current = &head;
	memset(buf, 0, sizeof(buf));
	while (fgets(buf, sizeof(buf), fd) != NULL) {
		uint64_t tmp;
		struct linked_list *p = NULL;

		ret = strtouint64(buf, 10, &tmp);
		if (ret != PQOS_RETVAL_OK)
			goto resctrl_alloc_task_read_exit_clean;
		p = malloc(sizeof(head));
		if (p == NULL)
			goto resctrl_alloc_task_read_exit_clean;
		p->task_id = tmp;
		p->next = NULL;
		current->next = p;
		current = p;
		idx++;
	}

	/* if no pids found then allocate empty buffer to be returned */
	if (idx == 0)
		tasks = (unsigned *) calloc(1, sizeof(tasks[0]));
	else
		tasks = (unsigned *) malloc(idx * sizeof(tasks[0]));
	if (tasks == NULL)
		goto resctrl_alloc_task_read_exit_clean;

	*count = idx;
	current = head.next;
	idx = 0;
	while (current != NULL) {
		tasks[idx++] = current->task_id;
		current = current->next;
	}

 resctrl_alloc_task_read_exit_clean:
	resctrl_alloc_fclose(fd);
	current = head.next;
	while (current != NULL) {
		struct linked_list *tmp = current->next;

		free(current);
		current = tmp;
	}
	return tasks;
}

int
resctrl_alloc_task_search(unsigned *class_id,
                          const struct pqos_cap *cap,
                          const pid_t task)
{
	FILE *fd;
	unsigned i, max_cos = 0;
	int ret;

	/* Check if task exists */
	ret = resctrl_alloc_task_validate(task);
	if (ret != PQOS_RETVAL_OK)
		return PQOS_RETVAL_PARAM;

	/* Get number of COS */
	ret = resctrl_alloc_get_grps_num(cap, &max_cos);
	if (ret != PQOS_RETVAL_OK)
		return ret;

	/**
	 * Starting at highest COS - search all COS tasks files for task ID
	 */
	for (i = (max_cos - 1); (int)i >= 0; i--) {
		uint64_t tid = 0;
		char buf[128];

		/* Open resctrl tasks file */
		fd = resctrl_alloc_fopen(i, rctl_tasks, "r");
		if (fd == NULL)
			return PQOS_RETVAL_ERROR;

		/* Search tasks file for specified task ID */
		memset(buf, 0, sizeof(buf));
		while (fgets(buf, sizeof(buf), fd) != NULL) {
			ret = strtouint64(buf, 10, &tid);
			if (ret != PQOS_RETVAL_OK)
				continue;

			if (task == (pid_t)tid) {
				*class_id = i;
				if (resctrl_alloc_fclose(fd) != PQOS_RETVAL_OK)
					return PQOS_RETVAL_ERROR;

				return PQOS_RETVAL_OK;
			}
		}
		if (resctrl_alloc_fclose(fd) != PQOS_RETVAL_OK)
			return PQOS_RETVAL_ERROR;
	}
	/* If not found in any COS group - return error */
	LOG_ERROR("Failed to get association for task %d!\n", (int)task);
	return PQOS_RETVAL_ERROR;
}

int
resctrl_alloc_task_file_check(const unsigned class_id, unsigned *found)
{
	FILE *fd;
	char buf[128];

	/* Open resctrl tasks file */
	fd = resctrl_alloc_fopen(class_id, rctl_tasks, "r");
	if (fd == NULL)
		return PQOS_RETVAL_ERROR;

	/* Search tasks file for any task ID */
	memset(buf, 0, sizeof(buf));
	if (fgets(buf, sizeof(buf), fd) != NULL)
		*found = 1;

	if (resctrl_alloc_fclose(fd) != PQOS_RETVAL_OK)
		return PQOS_RETVAL_ERROR;

	return PQOS_RETVAL_OK;
}

