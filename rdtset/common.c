/*
 *   BSD LICENSE
 *
 *   Copyright(c) 2016 Intel Corporation. All rights reserved.
 *   All rights reserved.
 *
 *   Redistribution and use in source and binary forms, with or without
 *   modification, are permitted provided that the following conditions
 *   are met:
 *
 *     * Redistributions of source code must retain the above copyright
 *       notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above copyright
 *       notice, this list of conditions and the following disclaimer in
 *       the documentation and/or other materials provided with the
 *       distribution.
 *     * Neither the name of Intel Corporation nor the names of its
 *       contributors may be used to endorse or promote products derived
 *       from this software without specific prior written permission.
 *
 *   THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 *   "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 *   LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 *   A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 *   OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 *   SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 *   LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 *   DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 *   THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 *   (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 *   OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>

#include "common.h"
#include "rdt.h"

int
str_to_cpuset(const char *cpustr, const unsigned cpustr_len, cpu_set_t *cpuset)
{
	unsigned idx, min, max;
	char *buff = malloc(cpustr_len + 1);
	char *end = NULL;
	const char *str = buff;
	int ret = 0;

	if (NULL == buff || NULL == cpustr || NULL == cpuset || 0 == cpustr_len)
		goto err;

	memcpy(buff, cpustr, cpustr_len);
	buff[cpustr_len] = 0;
	CPU_ZERO(cpuset);

	while (isblank(*str))
		str++;

	/* only digit is qualify for start point */
	if (!isdigit(*str) || *str == '\0')
		goto err;

	min = CPU_SETSIZE;
	do {
		/* go ahead to the first digit */
		while (isblank(*str))
			str++;

		if (!isdigit(*str))
			goto err;

		/* get the digit value */
		errno = 0;
		idx = strtoul(str, &end, 10);
		if (errno != 0 || end == NULL || end == str ||
				idx >= CPU_SETSIZE)
			goto err;

		/* go ahead to separator '-',',' */
		while (isblank(*end))
			end++;

		if (*end == '-') {
			if (min == CPU_SETSIZE)
				min = idx;
			else /* avoid continuous '-' */
				goto err;
		} else if (*end == ',' || *end == 0) {
			max = idx;

			if (min == CPU_SETSIZE)
				min = idx;

			for (idx = MIN(min, max); idx <= MAX(min, max);
					idx++)
				CPU_SET(idx, cpuset);

			min = CPU_SETSIZE;
		} else
			goto err;

		str = end + 1;
	} while (*end != '\0');

	ret = end - buff;

	free(buff);
	return ret;

err:
	if (buff != NULL)
		free(buff);
	return -EINVAL;
}

void
cpuset_to_str(char *cpustr, const unsigned cpustr_len,
		const cpu_set_t *cpuset)
{
	unsigned len = 0, j = 0;

	memset(cpustr, 0, cpustr_len);

	/* Generate CPU list */
	for (j = 0; j < CPU_SETSIZE; j++) {
		if (CPU_ISSET(j, cpuset) != 1)
			continue;

		len += snprintf(cpustr + len, cpustr_len - len - 1, "%u,", j);

		if (len >= cpustr_len - 1) {
			len = cpustr_len;
			memcpy(cpustr + cpustr_len - 4, "...", 3);
			break;
		}
	}

	/* Remove trailing separator */
	cpustr[len-1] = 0;
}

/**
 * @brief Function to check if a value is already contained in a table
 *
 * @param tab table of values to check
 * @param size size of the table
 * @param val value to search for
 *
 * @return If the value is already in the table
 * @retval 1 if value if found
 * @retval 0 if value is not found
 */
static int
isdup(const uint64_t *tab, const unsigned size, const uint64_t val)
{
        unsigned i;

        for (i = 0; i < size; i++)
                if (tab[i] == val)
                        return 1;
        return 0;
}

/**
 * @brief Converts string into 64-bit unsigned number.
 *
 * Numbers can be in decimal or hexadecimal format.
 * On error, this functions causes process to exit with FAILURE code.
 *
 * @param s string to be converted into 64-bit unsigned number
 *
 * @return Numeric value of the string representing the number
 */
static uint64_t
strtouint64(const char *s)
{
        const char *str = s;
        int base = 10;
        uint64_t n = 0;
        char *endptr = NULL;

        if (s == NULL)
                exit(EXIT_FAILURE);

        if (strncasecmp(s, "0x", 2) == 0) {
                base = 16;
                s += 2;
        }

        n = strtoull(s, &endptr, base);

        if (!(*s != '\0' && *endptr == '\0')) {
                printf("Error converting '%s' to unsigned number!\n", str);
                exit(EXIT_FAILURE);
        }

        return n;
}

unsigned
strlisttotab(char *s, uint64_t *tab, const unsigned max)
{
        unsigned index = 0;
        char *saveptr = NULL;

        if (s == NULL || tab == NULL || max == 0)
                return index;

        for (;;) {
                char *p = NULL;
                char *token = NULL;

                token = strtok_r(s, ",", &saveptr);
                if (token == NULL)
                        break;

                s = NULL;

                /* get rid of leading spaces & skip empty tokens */
                while (isspace(*token))
                        token++;
                if (*token == '\0')
                        continue;

                p = strchr(token, '-');
                if (p != NULL) {
                        /**
                         * range of numbers provided
                         * example: 1-5 or 12-9
                         */
                        uint64_t n, start, end;
                        *p = '\0';
                        start = strtouint64(token);
                        end = strtouint64(p+1);
                        if (start > end) {
                                /**
                                 * no big deal just swap start with end
                                 */
                                n = start;
                                start = end;
                                end = n;
                        }
                        for (n = start; n <= end; n++) {
                                if (!(isdup(tab, index, n))) {
                                        tab[index] = n;
                                        index++;
                                }
                                if (index >= max)
                                        return index;
                        }
                } else {
                        /**
                         * single number provided here
                         * remove duplicates if necessary
                         */
                        uint64_t val = strtouint64(token);

                        if (!(isdup(tab, index, val))) {
                                tab[index] = val;
                                index++;
                        }
                        if (index >= max)
                                return index;
                }
        }

        return index;
}
