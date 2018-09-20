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
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

/**
 * @brief  Set of utility functions to list and retrieve L3CA setting profiles
 */
#include <stdio.h>
#include <string.h>

#include "profiles.h"
#include "pqos.h"
#include "main.h"
#include "alloc.h"

#define PROFILES_MIN_COS 4

/**
 * 11-cache ways
 */
static const char * const classes_way11_overlapN_equalY[] = {
        "0=0x007",
        "1=0x038",
        "2=0x1C0",
        "3=0x600"
};

static const char * const classes_way11_overlapN_equalN[] = {
        "0=0x01F",
        "1=0x060",
        "2=0x180",
        "3=0x600"
};

static const char * const classes_way11_overlapP0_equalN[] = {
        "0=0x7FF",
        "1=0x060",
        "2=0x180",
        "3=0x600"
};

static const char * const classes_way11_overlapY_equalN[] = {
        "0=0x7FF",
        "1=0x7F0",
        "2=0x700",
        "3=0x600"
};

/**
 * 12-cache ways
 */
static const char * const classes_way12_overlapN_equalY[] = {
        "0=0x007",
        "1=0x038",
        "2=0x1C0",
        "3=0xE00"
};

static const char * const classes_way12_overlapN_equalN[] = {
        "0=0x03F",
        "1=0x0C0",
        "2=0x300",
        "3=0xC00"
};

static const char * const classes_way12_overlapP0_equalN[] = {
        "0=0xFFF",
        "1=0x0C0",
        "2=0x300",
        "3=0xC00"
};

static const char * const classes_way12_overlapY_equalN[] = {
        "0=0xFFF",
        "1=0xFF0",
        "2=0xF00",
        "3=0xC00"
};

/**
 * 16-cache ways
 */
static const char * const classes_way16_overlapN_equalY[] = {
        "0=0x000F",
        "1=0x00F0",
        "2=0x0F00",
        "3=0xF000"
};

static const char * const classes_way16_overlapN_equalN[] = {
        "0=0x03FF",
        "1=0x0C00",
        "2=0x3000",
        "3=0xC000"
};

static const char * const classes_way16_overlapP0_equalN[] = {
        "0=0xFFFF",
        "1=0x0C00",
        "2=0x3000",
        "3=0xC000"
};

static const char * const classes_way16_overlapY_equalN[] = {
        "0=0xFFFF",
        "1=0xFF00",
        "2=0xF000",
        "3=0xC000"
};

/**
 * 20-cache ways
 */
static const char * const classes_way20_overlapN_equalY[] = {
        "0=0x0001F",
        "1=0x003E0",
        "2=0x07C00",
        "3=0xF8000"
};

static const char * const classes_way20_overlapN_equalN[] = {
        "0=0x000FF",
        "1=0x00F00",
        "2=0x0F000",
        "3=0xF000"
};

static const char * const classes_way20_overlapP0_equalN[] = {
        "0=0xFFFFF",
        "1=0x0C000",
        "2=0x30000",
        "3=0xC0000"
};

static const char * const classes_way20_overlapY_equalN[] = {
        "0=0xFFFFF",
        "1=0xFF000",
        "2=0xF0000",
        "3=0xC0000"
};

/**
 * meat and potatos now :)
 */
struct llc_allocation_config {
        unsigned num_ways;
        unsigned num_classes;
        const char * const *tab;
};

static const struct llc_allocation_config config_cfg0[] = {
        { .num_ways = 11,
          .num_classes = 4,
          .tab = classes_way11_overlapN_equalY,
        },
        { .num_ways = 12,
          .num_classes = 4,
          .tab = classes_way12_overlapN_equalY,
        },
        { .num_ways = 16,
          .num_classes = 4,
          .tab = classes_way16_overlapN_equalY,
        },
        { .num_ways = 20,
          .num_classes = 4,
          .tab = classes_way20_overlapN_equalY,
        }
};

static const struct llc_allocation_config config_cfg1[] = {
        { .num_ways = 11,
          .num_classes = 4,
          .tab = classes_way11_overlapN_equalN,
        },
        { .num_ways = 12,
          .num_classes = 4,
          .tab = classes_way12_overlapN_equalN,
        },
        { .num_ways = 16,
          .num_classes = 4,
          .tab = classes_way16_overlapN_equalN,
        },
        { .num_ways = 20,
          .num_classes = 4,
          .tab = classes_way20_overlapN_equalN,
        }
};

static const struct llc_allocation_config config_cfg2[] = {
        { .num_ways = 11,
          .num_classes = 4,
          .tab = classes_way11_overlapP0_equalN,
        },
        { .num_ways = 12,
          .num_classes = 4,
          .tab = classes_way12_overlapP0_equalN,
        },
        { .num_ways = 16,
          .num_classes = 4,
          .tab = classes_way16_overlapP0_equalN,
        },
        { .num_ways = 20,
          .num_classes = 4,
          .tab = classes_way20_overlapP0_equalN,
        },
};

static const struct llc_allocation_config config_cfg3[] = {
        { .num_ways = 11,
          .num_classes = 4,
          .tab = classes_way11_overlapY_equalN,
        },
        { .num_ways = 12,
          .num_classes = 4,
          .tab = classes_way12_overlapY_equalN,
        },
        { .num_ways = 16,
          .num_classes = 4,
          .tab = classes_way16_overlapY_equalN,
        },
        { .num_ways = 20,
          .num_classes = 4,
          .tab = classes_way20_overlapY_equalN,
        },
};

struct llc_allocation {
        const char *id;
        const char *descr;
        unsigned num_config;
        const struct llc_allocation_config *config;
};

static const struct llc_allocation allocation_tab[] = {
        { .id = "CFG0",
          .descr = "non-overlapping, ways equally divided",
          .num_config = DIM(config_cfg0),
          .config = config_cfg0,
        },
        { .id = "CFG1",
          .descr = "non-overlapping, ways unequally divided",
          .num_config = DIM(config_cfg1),
          .config = config_cfg1,
        },
        { .id = "CFG2",
          .descr = "overlapping, ways unequally divided, "
          "class 0 can access all ways",
          .num_config = DIM(config_cfg2),
          .config = config_cfg2,
        },
        { .id = "CFG3",
          .descr = "ways unequally divided, overlapping access "
          "for higher classes",
          .num_config = DIM(config_cfg3),
          .config = config_cfg3,
        },
};

void profile_l3ca_list(FILE *fp)
{
        unsigned i = 0, j = 0;

        ASSERT(fp != NULL);
        if (fp == NULL)
                return;

        for (i = 0; i < DIM(allocation_tab); i++) {
                const struct llc_allocation *ap = &allocation_tab[i];

                fprintf(fp,
                        "%u)\n"
                        "      Config ID: %s\n"
                        "    Description: %s\n"
                        " Configurations:\n",
                        i+1, ap->id, ap->descr);
                for (j = 0; j < ap->num_config; j++) {
                        fprintf(fp,
                                "\tnumber of classes = %u, number of cache ways = %u\n",
                                (unsigned) ap->config[j].num_classes,
                                (unsigned) ap->config[j].num_ways);
                }
        }
}

/**
 * @brief Retrieves selected L3CA profile by its \a id
 *
 * @param [in] id profile identity (string)
 * @param [in] l3ca L3CA capability structure
 * @param [out] p_num number of L3CA classes of service retrieved for
 *              the profile
 * @param [out] p_tab pointer to definition of L3CA classes of service
 *
 * @return Operations status
 * @retval PQOS_RETVAL_OK on success
 */
static int
profile_l3ca_get(const char *id, const struct pqos_cap_l3ca *l3ca,
                 unsigned *p_num, const char * const **p_tab)
{
        unsigned i = 0, j = 0;

        ASSERT(id != NULL);
        ASSERT(l3ca != NULL);
        ASSERT(p_tab != NULL);
        ASSERT(p_num != NULL);

        if (id == NULL || l3ca == NULL || p_tab == NULL || p_num == NULL)
                return PQOS_RETVAL_PARAM;

	if (l3ca->num_classes < PROFILES_MIN_COS)
	        return PQOS_RETVAL_RESOURCE;

        for (i = 0; i < DIM(allocation_tab); i++) {
                const struct llc_allocation *ap = &allocation_tab[i];

                if (strcasecmp(id, ap->id) != 0)
                        continue;
                for (j = 0; j < ap->num_config; j++) {
		        /* no need to check number of classes here */
                        if (ap->config[j].num_ways != l3ca->num_ways)
                                continue;
                        *p_num = ap->config[j].num_classes;
                        *p_tab = ap->config[j].tab;
                        return PQOS_RETVAL_OK;
                }
        }

        return PQOS_RETVAL_RESOURCE;
}

int
profile_l3ca_apply(const char *name,
                   const struct pqos_capability *cap_l3ca)
{
        unsigned cnum = 0;
        const char * const *cptr = NULL;

        if (cap_l3ca != NULL &&
            profile_l3ca_get(name, cap_l3ca->u.l3ca, &cnum,
                             &cptr) == PQOS_RETVAL_OK) {
                /**
                 * All profile classes are defined as strings
                 * in format that is command line friendly.
                 *
                 * This effectively simulates series of -e command
                 * line options. "llc:" is glued to each of the strings
                 * so that profile class definitions don't have to
                 * include it.
                 */
                char cb[64];
                unsigned i = 0, offset = 0;

                memset(cb, 0, sizeof(cb));
                strcpy(cb, "llc:");
                offset = (unsigned)strlen("llc:");

                for (i = 0; i < cnum; i++) {
                        strncpy(cb+offset, cptr[i],
                                sizeof(cb)-1-offset);
                        selfn_allocation_class(cb);
                }
        } else {
                printf("Allocation profile '%s' not found or "
                       "cache allocation not supported!\n",
                       name);
                return -1;
        }

        return 0;
}
