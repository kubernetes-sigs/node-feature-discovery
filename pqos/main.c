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
 * @brief Platform QoS utility - main module
 *
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <ctype.h>                                      /**< isspace() */
#include <sys/types.h>                                  /**< open() */
#include <sys/stat.h>
#include <fcntl.h>
#include <getopt.h>                                     /**< getopt_long() */

#include "pqos.h"

#include "main.h"
#include "profiles.h"
#include "monitor.h"
#include "alloc.h"
#include "cap.h"

/**
 * Default CDP configuration option - don't enforce on or off
 */
static enum pqos_cdp_config selfn_l3cdp_config = PQOS_REQUIRE_CDP_ANY;

/**
 * Monitoring reset
 */
static int sel_mon_reset = 0;

/**
 * Maintains pointer to selected log file name
 */
static char *sel_log_file = NULL;

/**
 * Maintains pointer to selected config file
 */
static char *sel_config_file = NULL;

/**
 * Maintains pointer to allocation profile from internal DB
 */
static char *sel_allocation_profile = NULL;

/**
 * Maintains verbose mode choice selected in config string
 */
static int sel_verbose_mode = 0;

/**
 * Reset allocation configuration
 */
static int sel_reset_alloc = 0;

/**
 * Enable showing cache allocation settings
 */
static int sel_show_allocation_config = 0;

/**
 * Enable displaying supported RDT capabilities
 */
static int sel_display = 0;

/**
 * Enable displaying supported RDT capabilities in verbose mode
 */
static int sel_display_verbose = 0;

/**
 * Selected library interface
 */
int sel_interface = PQOS_INTER_MSR;

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

uint64_t strtouint64(const char *s)
{
        const char *str = s;
        int base = 10;
        uint64_t n = 0;
        char *endptr = NULL;

        ASSERT(s != NULL);

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

unsigned strlisttotab(char *s, uint64_t *tab, const unsigned max)
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

__attribute__ ((noreturn)) void
parse_error(const char *arg, const char *note)
{
        printf("Error parsing \"%s\" command line argument. %s\n",
               arg ? arg : "<null>",
               note ? note : "");
        exit(EXIT_FAILURE);
}

void selfn_strdup(char **sel, const char *arg)
{
        ASSERT(sel != NULL && arg != NULL);
        if (*sel != NULL) {
                free(*sel);
                *sel = NULL;
        }
        *sel = strdup(arg);
        ASSERT(*sel != NULL);
        if (*sel == NULL) {
                printf("String duplicate error!\n");
                exit(EXIT_FAILURE);
        }
}

/**
 * @brief Function to print warning to users as utility begins
 */
static void
print_warning(void)
{
#ifdef __linux__
        printf("NOTE:  Mixed use of MSR and kernel interfaces "
               "to manage\n       CAT or CMT & MBM may lead to "
               "unexpected behavior.\n");
#endif
}

/**
 * @brief Selects log file
 *
 * @param arg string passed to -l command line option
 */
static void
selfn_log_file(const char *arg)
{
        selfn_strdup(&sel_log_file, arg);
}

/**
 * @brief Selects verbose mode on
 *
 * @param arg not used
 */
static void
selfn_verbose_mode(const char *arg)
{
        UNUSED_ARG(arg);
        sel_verbose_mode = 1;
}

/**
 * @brief Selects super verbose mode on
 *
 * @param arg not used
 */
static void
selfn_super_verbose_mode(const char *arg)
{
        UNUSED_ARG(arg);
        sel_verbose_mode = 2;
}

/**
 * @brief Sets allocation reset flag
 *
 * @param [in] arg optional configuration string
 *             if NULL or zero length  then configuration check is skipped
 */
static void selfn_reset_alloc(const char *arg)
{
        if (arg != NULL && (strlen(arg) > 0)) {
                const struct {
                        const char *name;
                        enum pqos_cdp_config cdp;
                } patterns[] = {
                        {"l3cdp-on",  PQOS_REQUIRE_CDP_ON},
                        {"l3cdp-off", PQOS_REQUIRE_CDP_OFF},
                        {"l3cdp-any", PQOS_REQUIRE_CDP_ANY},
                };
                unsigned i;

                for (i = 0; i < DIM(patterns); i++)
                        if (strcasecmp(arg, patterns[i].name) == 0)
                                break;

                if (i >= DIM(patterns)) {
                        printf("Unrecognized '%s' allocation "
                               "reset option!\n", arg);
                        exit(EXIT_FAILURE);
                }
                selfn_l3cdp_config = patterns[i].cdp;
        }
        sel_reset_alloc = 1;
}

/**
 * @brief Selects showing allocation settings
 *
 * @param arg not used
 */
static void selfn_show_allocation(const char *arg)
{
        UNUSED_ARG(arg);
        sel_show_allocation_config = 1;
}

/**
 * @brief Selects displaying supported capabilities
 *
 * @param arg not used
 */
static void selfn_display(const char *arg)
{
        UNUSED_ARG(arg);
        sel_display = 1;
}

/**
 * @brief Selects displaying supported capabilities in verbose mode
 *
 * @param arg not used
 */
static void selfn_display_verbose(const char *arg)
{
        UNUSED_ARG(arg);
        sel_display_verbose = 1;
}

/**
 * @brief Selects allocation profile from internal DB
 *
 * @param arg string passed to -c command line option
 */
static void
selfn_allocation_select(const char *arg)
{
        selfn_strdup(&sel_allocation_profile, arg);
}

/**
 * @brief Selects library OS interface
 *
 * @param arg not used
 */
static void
selfn_iface_os(const char *arg)
{
        UNUSED_ARG(arg);
        sel_interface = PQOS_INTER_OS;
}

/**
 * @brief Opens configuration file and parses its contents
 *
 * @param fname Name of the file with configuration parameters
 */
static void
parse_config_file(const char *fname)
{
        if (fname == NULL)
                parse_error("-f", "Invalid configuration file name!\n");

        static const struct {
                const char *option;
                void (*fn)(const char *);
        } optab[] = {
                {"show-alloc:",         selfn_show_allocation },   /**< -s */
                {"display:",            selfn_display },           /**< -d */
                {"display-verbose:",    selfn_display_verbose },   /**< -D */
                {"log-file:",           selfn_log_file },          /**< -l */
                {"verbose-mode:",       selfn_verbose_mode },      /**< -v */
                {"super-verbose-mode:", selfn_super_verbose_mode },/**< -V */
                {"alloc-class-set:",    selfn_allocation_class },  /**< -e */
                {"alloc-assoc-set:",    selfn_allocation_assoc },  /**< -a */
                {"alloc-class-select:", selfn_allocation_select }, /**< -c */
                {"monitor-pids:",       selfn_monitor_pids },      /**< -p */
                {"monitor-cores:",      selfn_monitor_cores },     /**< -m */
                {"monitor-time:",       selfn_monitor_time },      /**< -t */
                {"monitor-interval:",   selfn_monitor_interval },  /**< -i */
                {"monitor-file:",       selfn_monitor_file },      /**< -o */
                {"monitor-file-type:",  selfn_monitor_file_type }, /**< -u */
                {"monitor-top-like:",   selfn_monitor_top_like },  /**< -T */
                {"reset-cat:",          selfn_reset_alloc },       /**< -R */
                {"iface-os:",           selfn_iface_os },          /**< -I */
        };
        FILE *fp = NULL;
        char cb[256];

        fp = fopen(fname, "r");
        if (fp == NULL)
                parse_error(fname, "cannot open configuration file!");

        memset(cb, 0, sizeof(cb));

        while (fgets(cb, sizeof(cb)-1, fp) != NULL) {
                int i, j, remain;
                char *cp = NULL;

                for (j = 0; j < (int)sizeof(cb)-1; j++)
                        if (!isspace(cb[j]))
                                break;

                if (j >= (int)(sizeof(cb)-1))
                        continue; /**< blank line */

                if (strlen(cb+j) == 0)
                        continue; /**< blank line */

                if (cb[j] == '#')
                        continue; /**< comment */

                cp = cb+j;
                remain = (int)strlen(cp);

                /**
                 * remove trailing white spaces
                 */
                for (i = (int)strlen(cp)-1; i > 0; i--)
                        if (!isspace(cp[i])) {
                                cp[i+1] = '\0';
                                break;
                        }

                for (i = 0; i < (int)DIM(optab); i++) {
                        int len = (int)strlen(optab[i].option);

                        if (len > remain)
                                continue;

                        if (strncasecmp(cp, optab[i].option, (size_t)len) != 0)
                                continue;

                        while (isspace(cp[len]))
                                len++; /**< skip space characters */

                        optab[i].fn(cp+len);
                        break;
                }

                if (i >= (int)DIM(optab))
                        parse_error(cp,
                                    "Unrecognized configuration file command");
        }

        fclose(fp);
}

static const char *m_cmd_name = "pqos";                     /**< command name */
static const char help_printf_short[] =
        "Usage: %s [-h] [--help] [-v] [--verbose] [-V] [--super-verbose]\n"
        "          [-l FILE] [--log-file=FILE] [-I] [--iface-os]\n"
        "       %s [-s] [--show]\n"
        "       %s [-d] [--display] [-D] [--display-verbose]\n"
        "       %s [-m EVTCORES] [--mon-core=EVTCORES] | [-p [EVTPIDS]] "
        "[--mon-pid[=EVTPIDS]]\n"
        "          [-t SECONDS] [--mon-time=SECONDS]\n"
        "          [-i N] [--mon-interval=N]\n"
        "          [-T] [--mon-top]\n"
        "          [-o FILE] [--mon-file=FILE]\n"
        "          [-u TYPE] [--mon-file-type=TYPE]\n"
        "          [-r] [--mon-reset]\n"
        "       %s [-e CLASSDEF] [--alloc-class=CLASSDEF]\n"
        "          [-a CLASS2ID] [--alloc-assoc=CLASS2ID]\n"
        "       %s [-R] [--alloc-reset]\n"
        "       %s [-H] [--profile-list] | [-c PROFILE] "
        "[--profile-set=PROFILE]\n"
        "       %s [-f FILE] [--config-file=FILE]\n";

static const char help_printf_long[] =
        "Description:\n"
        "  -h, --help                  help page\n"
        "  -v, --verbose               verbose mode\n"
        "  -V, --super-verbose         super-verbose mode\n"
        "  -s, --show                  show current PQoS configuration\n"
        "  -d, --display               display supported capabilities\n"
        "  -D, --display-verbose       display supported capabilities in verbose mode\n"
        "  -f FILE, --config-file=FILE load commands from selected file\n"
        "  -l FILE, --log-file=FILE    log messages into selected file\n"
        "  -e CLASSDEF, --alloc-class=CLASSDEF\n"
        "          define allocation classes.\n"
        "          CLASSDEF format is 'TYPE:ID=DEFINITION;'.\n"
        "          To specify specific resources 'TYPE[@RESOURCE_ID]:ID=DEFINITION;'.\n"
        "          Examples: 'llc:0=0xffff;llc:1=0x00ff;llc@0-1:2=0xff00',\n"
	"                    'llc:0d=0xfff;llc:0c=0xfff00',\n"
        "                    'l2:2=0x3f;l2@2:1=0xf',\n"
        "                    'mba:1=30;mba@1:3=80'.\n"
        "  -a CLASS2ID, --alloc-assoc=CLASS2ID\n"
        "          associate cores/tasks with an allocation class.\n"
        "          CLASS2ID format is 'TYPE:ID=CORE_LIST/TASK_LIST'.\n"
        "          Example 'llc:0=0,2,4,6-10;llc:1=1'.\n"
        "          Example 'core:0=0,2,4,6-10;core:1=1'.\n"
        "          Example 'pid:0=3543,7643,4556;pid:1=7644'.\n"
        "  -R [CONFIG], --alloc-reset[=CONFIG]\n"
        "          reset allocation configuration (L2/L3 CAT & MBA)\n"
        "          CONFIG can be: l3cdp-on, l3cdp-off or l3cdp-any (default).\n"
        "  -m EVTCORES, --mon-core=EVTCORES\n"
        "          select cores and events for monitoring.\n"
        "          EVTCORES format is 'EVENT:CORE_LIST'.\n"
        "          Example: \"all:0,2,4-10;llc:1,3;mbr:11-12\".\n"
        "          Cores can be grouped by enclosing them in square brackets,\n"
        "          example: \"llc:[0-3];all:[4,5,6];mbr:[0-3],7,8\".\n"
        "  -p [EVTPIDS], --mon-pid[=EVTPIDS]\n"
        "          select top 10 most active (CPU utilizing) process ids to monitor\n"
        "          or select process ids and events to monitor.\n"
        "          EVTPIDS format is 'EVENT:PID_LIST'.\n"
        "          Example 'llc:22,25673' or 'all:892,4588-4592'.\n"
        "          Note: processes and cores cannot be monitored together.\n"
        "                Requires Linux and kernel versions 4.1 and newer.\n"
        "  -o FILE, --mon-file=FILE    output monitored data in a FILE\n"
        "  -u TYPE, --mon-file-type=TYPE\n"
        "          select output file format type for monitored data.\n"
        "          TYPE is one of: text (default), xml or csv.\n"
        "  -i N, --mon-interval=N      set sampling interval to Nx100ms,\n"
        "                              default 10 = 10 x 100ms = 1s.\n"
        "  -T, --mon-top               top like monitoring output\n"
        "  -t SECONDS, --mon-time=SECONDS\n"
        "          set monitoring time in seconds. Use 'inf' or 'infinite'\n"
        "          for infinite monitoring. CTRL+C stops monitoring.\n"
        "  -r, --mon-reset             monitoring reset, claim all RMID's\n"
        "  -H, --profile-list          list supported allocation profiles\n"
        "  -c PROFILE, --profile-set=PROFILE\n"
        "          select a PROFILE of predefined allocation classes.\n"
        "          Use -H to list available profiles.\n"
        "  -I, --iface-os\n"
        "          set the library interface to use the kernel\n"
        "          implementation. If not set the default implementation is\n"
        "          to program the MSR's directly.\n";

/**
 * @brief Displays help information
 *
 * @param is_long print long help version or a short one
 *
 */
static void print_help(const int is_long)
{
        printf(help_printf_short,
               m_cmd_name, m_cmd_name, m_cmd_name, m_cmd_name, m_cmd_name,
               m_cmd_name, m_cmd_name, m_cmd_name);
        if (is_long)
                printf(help_printf_long);
}

static struct option long_cmd_opts[] = {
        {"help",            no_argument,       0, 'h'},
        {"log-file",        required_argument, 0, 'l'},
        {"config-file",     required_argument, 0, 'f'},
        {"show",            no_argument,       0, 's'},
        {"display",         no_argument,       0, 'd'},
        {"display-verbose", no_argument,       0, 'D'},
        {"profile-list",    no_argument,       0, 'H'},
        {"profile-set",     required_argument, 0, 'c'},
        {"mon-interval",    required_argument, 0, 'i'},
        {"mon-pid",         required_argument, 0, 'p'},
        {"mon-core",        required_argument, 0, 'm'},
        {"mon-time",        required_argument, 0, 't'},
        {"mon-top",         no_argument,       0, 'T'},
        {"mon-file",        required_argument, 0, 'o'},
        {"mon-file-type",   required_argument, 0, 'u'},
        {"mon-reset",       no_argument,       0, 'r'},
        {"alloc-class",     required_argument, 0, 'e'},
        {"alloc-reset",     required_argument, 0, 'R'},
        {"alloc-assoc",     required_argument, 0, 'a'},
        {"verbose",         no_argument,       0, 'v'},
        {"super-verbose",   no_argument,       0, 'V'},
        {"iface-os",        no_argument,       0, 'I'},
        {0, 0, 0, 0} /* end */
};

int main(int argc, char **argv)
{
        struct pqos_config cfg;
        const struct pqos_cpuinfo *p_cpu = NULL;
        const struct pqos_cap *p_cap = NULL;
        const struct pqos_capability *cap_mon = NULL, *cap_l3ca = NULL,
                *cap_l2ca = NULL, *cap_mba = NULL;
        unsigned sock_count, *sockets = NULL;
        int cmd, ret, exit_val = EXIT_SUCCESS;
        int opt_index = 0, pid_flag = 0;

        m_cmd_name = argv[0];
        print_warning();

        memset(&cfg, 0, sizeof(cfg));

        while ((cmd = getopt_long(argc, argv,
                                  ":Hhf:i:m:Tt:l:o:u:e:c:a:p:sdDrvVIR:",
                                  long_cmd_opts, &opt_index)) != -1) {
                switch (cmd) {
                case 'h':
                        print_help(1);
                        return EXIT_SUCCESS;
                case 'H':
                        profile_l3ca_list(stdout);
                        return EXIT_SUCCESS;
                case 'f':
                        if (sel_config_file != NULL) {
                                printf("Only one config file argument is "
                                       "accepted!\n");
                                return EXIT_FAILURE;
                        }
                        selfn_strdup(&sel_config_file, optarg);
                        parse_config_file(sel_config_file);
                        break;
                case 'i':
                        selfn_monitor_interval(optarg);
                        break;
                case 'p':
                        if (optarg != NULL && *optarg == '-') {
                                /**
                                 * Next switch option wrongly assumed to be
                                 * argument to '-p'.
                                 * In order to fix it, we are handling this as
                                 * '-p' without parameters (as it should be)
                                 * to start top-pids monitoring mode.
                                 * Have to rewind \a optind as well.
                                 */
                                selfn_monitor_top_pids();
                                optind--;
                                break;
                        }
                        selfn_monitor_pids(optarg);
                        pid_flag = 1;
                        break;
                case 'm':
                        selfn_monitor_cores(optarg);
                        break;
                case 't':
                        selfn_monitor_time(optarg);
                        break;
                case 'T':
                        selfn_monitor_top_like(NULL);
                        break;
                case 'l':
                        selfn_log_file(optarg);
                        break;
                case 'o':
                        selfn_monitor_file(optarg);
                        break;
                case 'u':
                        selfn_monitor_file_type(optarg);
                        break;
                case 'e':
                        selfn_allocation_class(optarg);
                        break;
                case 'r':
                        sel_mon_reset = 1;
                        break;
                case 'R':
                        if (optarg != NULL) {
                                if (*optarg == '-') {
                                        /**
                                        * Next switch option wrongly assumed
                                        * to be argument to '-R'.
                                        * Pass NULL as argument to '-R' function
                                        * and rewind \a optind.
                                        */
                                        selfn_reset_alloc(NULL);
                                        optind--;
                                } else
                                        selfn_reset_alloc(optarg);
                        } else
                                selfn_reset_alloc(NULL);
                        break;
                case ':':
                        /**
                         * This is handler for missing mandatory argument
                         * (enabled by leading ':' in getopt() argument).
                         * -R and -p are only allowed switch for optional args.
                         * Other switches need to report error.
                         */
                        if (optopt == 'R') {
                                selfn_reset_alloc(NULL);
                        } else if (optopt == 'p') {
                                /**
                                 * Top pids mode - in case of '-I -p' top N
                                 * pids (by CPU usage) will be displayed and
                                 * monitored for cache/mbm/misses
                                 */
                                selfn_monitor_top_pids();
                                pid_flag = 1;
                        } else {
                                printf("Option -%c is missing required "
                                       "argument\n", optopt);
                                return EXIT_FAILURE;
                        }
                        break;
                case 'a':
                        selfn_allocation_assoc(optarg);
                        pid_flag |= alloc_pid_flag;
                        break;
                case 'c':
                        selfn_allocation_select(optarg);
                        break;
                case 's':
                        selfn_show_allocation(NULL);
                        break;
                case 'd':
                        selfn_display(NULL);
                        break;
                case 'D':
                        selfn_display_verbose(NULL);
                        break;
                case 'v':
                        selfn_verbose_mode(NULL);
                        break;
                case 'V':
                        selfn_super_verbose_mode(NULL);
                        break;
                case 'I':
                        selfn_iface_os(NULL);
                        break;
                default:
                        printf("Unsupported option: -%c. "
                               "See option -h for help.\n", optopt);
                        return EXIT_FAILURE;
                        break;
                case '?':
                        print_help(0);
                        return EXIT_SUCCESS;
                        break;
                }
        }

        if (pid_flag == 1 && sel_interface == PQOS_INTER_MSR) {
                printf("Error! OS interface option [-I] needed for PID"
                       " operations. Please re-run with the -I option.\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_1;
        }
        cfg.verbose = sel_verbose_mode;
        cfg.interface = sel_interface;
        /**
         * Set up file descriptor for message log
         */
        if (sel_log_file == NULL) {
                cfg.fd_log = STDOUT_FILENO;
        } else {
                cfg.fd_log = open(sel_log_file, O_WRONLY|O_CREAT,
                                  S_IRUSR|S_IWUSR);
                if (cfg.fd_log == -1) {
                        printf("Error opening %s log file!\n", sel_log_file);
                        exit_val = EXIT_FAILURE;
                        goto error_exit_1;
                }
        }

        ret = pqos_init(&cfg);
        if (ret != PQOS_RETVAL_OK) {
                printf("Error initializing PQoS library!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_1;
        }

        ret = pqos_cap_get(&p_cap, &p_cpu);
        if (ret != PQOS_RETVAL_OK) {
                printf("Error retrieving PQoS capabilities!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }

        sockets = pqos_cpu_get_sockets(p_cpu, &sock_count);
        if (sockets == NULL) {
                printf("Error retrieving CPU socket information!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }

        ret = pqos_cap_get_type(p_cap, PQOS_CAP_TYPE_MON, &cap_mon);
        if (ret == PQOS_RETVAL_PARAM) {
                printf("Error retrieving monitoring capabilities!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }

        ret = pqos_cap_get_type(p_cap, PQOS_CAP_TYPE_L3CA, &cap_l3ca);
        if (ret == PQOS_RETVAL_PARAM) {
                printf("Error retrieving L3 allocation capabilities!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }

        ret = pqos_cap_get_type(p_cap, PQOS_CAP_TYPE_L2CA, &cap_l2ca);
        if (ret == PQOS_RETVAL_PARAM) {
                printf("Error retrieving L2 allocation capabilities!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }

        ret = pqos_cap_get_type(p_cap, PQOS_CAP_TYPE_MBA, &cap_mba);
        if (ret == PQOS_RETVAL_PARAM) {
                printf("Error retrieving MB allocation capabilities!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }

        if (sel_mon_reset && cap_mon != NULL) {
                if (pqos_mon_reset() != PQOS_RETVAL_OK) {
                        exit_val = EXIT_FAILURE;
                        printf("CMT/MBM reset failed!\n");
                } else {
                        printf("CMT/MBM reset successful\n");
                }
        }

        if (sel_reset_alloc) {
                /**
                 * Reset allocation configuration to after-reset state and exit
                 */
                if (pqos_alloc_reset(selfn_l3cdp_config) != PQOS_RETVAL_OK) {
                        exit_val = EXIT_FAILURE;
                        printf("Allocation reset failed!\n");
                } else
                        printf("Allocation reset successful\n");
        }

        if (sel_show_allocation_config) {
                /**
                 * Show info about allocation config and exit
                 */
		alloc_print_config(cap_mon, cap_l3ca, cap_l2ca, cap_mba,
                                   sock_count, sockets, p_cpu,
                                   sel_verbose_mode);
                goto allocation_exit;
        }

        if (sel_display || sel_display_verbose) {
                /**
                 * Display info about supported capabilities
                 */
                cap_print_features(p_cap, p_cpu, sel_display_verbose);
                goto allocation_exit;
        }

        if (sel_allocation_profile != NULL) {
                if (profile_l3ca_apply(sel_allocation_profile, cap_l3ca) != 0) {
                        exit_val = EXIT_FAILURE;
                        goto error_exit_2;
                }
        }

        switch (alloc_apply(cap_l3ca, cap_l2ca, cap_mba, p_cpu)) {
        case 0: /* nothing to apply */
                break;
        case 1: /* new allocation config applied and all is good */
                goto allocation_exit;
                break;
        case -1: /* something went wrong */
        default:
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
                break;
        }

        /**
         * If -R was present ignore all monitoring related options
         */
        if (sel_reset_alloc)
                goto allocation_exit;

        /**
         * Just monitoring option left on the table now
         */
        if (cap_mon == NULL) {
                printf("Monitoring capability not detected!\n");
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }

        if (monitor_setup(p_cpu, cap_mon) != 0) {
                exit_val = EXIT_FAILURE;
                goto error_exit_2;
        }
        monitor_loop();
        monitor_stop();

 allocation_exit:
 error_exit_2:
        ret = pqos_fini();
        ASSERT(ret == PQOS_RETVAL_OK);
        if (ret != PQOS_RETVAL_OK)
                printf("Error shutting down PQoS library!\n");

 error_exit_1:
        monitor_cleanup();

        /**
         * Close file descriptor for message log
         */
        if (cfg.fd_log > 0 && cfg.fd_log != STDOUT_FILENO)
                close(cfg.fd_log);

        /**
         * Free allocated memory
         */
        if (sel_allocation_profile != NULL)
                free(sel_allocation_profile);
        if (sel_log_file != NULL)
                free(sel_log_file);
        if (sel_config_file != NULL)
                free(sel_config_file);
        if (sockets != NULL)
                free(sockets);

        return exit_val;
}
