###############################################################################
# Makefile script for rdtset tool
#
# @par
# BSD LICENSE
#
# Copyright(c) 2016 Intel Corporation. All rights reserved.
# All rights reserved.
#
# Redistribution and use in source and binary forms, with or without
# modification, are permitted provided that the following conditions
# are met:
#
#   * Redistributions of source code must retain the above copyright
#     notice, this list of conditions and the following disclaimer.
#   * Redistributions in binary form must reproduce the above copyright
#     notice, this list of conditions and the following disclaimer in
#     the documentation and/or other materials provided with the
#     distribution.
#   * Neither the name of Intel Corporation nor the names of its
#     contributors may be used to endorse or promote products derived
#     from this software without specific prior written permission.
#
# THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
# "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
# LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
# A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
# OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
# SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
# LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
# DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
# THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
# (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
# OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
#
###############################################################################

LIBDIR ?= ../lib
LDFLAGS = -L$(LIBDIR) -fPIE -z noexecstack -z relro -z now
LDLIBS = -lpqos -lpthread
CFLAGS = -I$(LIBDIR) \
	-W -Wall -Wextra -Wstrict-prototypes -Wmissing-prototypes \
	-Wmissing-declarations -Wold-style-definition -Wpointer-arith \
	-Wcast-qual -Wundef -Wwrite-strings \
	-Wformat -Wformat-security -fstack-protector -fPIE -D_FORTIFY_SOURCE=2 \
	-Wunreachable-code -Wmissing-noreturn -Wsign-compare -Wno-endif-labels \
	-D_GNU_SOURCE
ifneq ($(EXTRA_CFLAGS),)
CFLAGS += $(EXTRA_CFLAGS)
endif

# ICC and GCC options
ifeq ($(CC),icc)
else
CFLAGS += -Wcast-align -Wnested-externs
endif

# DEBUG build
ifeq ($(DEBUG),y)
CFLAGS += -g -ggdb -O0 -DDEBUG
else
CFLAGS += -g -O3
endif

# Needed by initgroups()
ifeq ($(shell uname), FreeBSD)
CFLAGS += -D_BSD_SOURCE
endif

# Build targets and dependencies
APP = rdtset
MAN = rdtset.8

# XXX: modify as desired
PREFIX ?= /usr/local
BIN_DIR = $(PREFIX)/bin
MAN_DIR = $(PREFIX)/man/man8

SRCS = $(sort $(wildcard *.c))
OBJS = $(SRCS:.c=.o)
DEPFILES = $(SRCS:.c=.d)

all: $(APP)

$(APP): $(OBJS)
	$(CC) $(LDFLAGS) $^ $(LDLIBS) -o $@

%.o: %.c %.d

%.d: %.c
	$(CC) -MM -MP -MF $@ $(CFLAGS) $<
	cat $@ | sed 's/$(@:.d=.o)/$@/' >> $@

.PHONY: clean install uninstall

install: $(APP) $(MAN)
ifeq ($(shell uname), FreeBSD)
	install -d $(BIN_DIR)
	install -d $(MAN_DIR)
	install -s $(APP) $(BIN_DIR)
	install -m 0444 $(MAN) $(MAN_DIR)
else
	install -D -s $(APP) $(BIN_DIR)/$(APP)
	install -m 0444 $(MAN) -D $(MAN_DIR)/$(MAN)
endif

uninstall:
	-rm $(BIN_DIR)/$(APP)
	-rm $(MAN_DIR)/$(MAN)

clean:
	-rm -f $(APP) $(OBJS) $(DEPFILES) ./*~

CHECKPATCH?=checkpatch.pl
.PHONY: style
style:
	$(CHECKPATCH) --no-tree --no-signoff --emacs \
	--ignore CODE_INDENT,INITIALISED_STATIC,LEADING_SPACE,SPLIT_STRING,NEW_TYPEDEFS,\
	UNSPECIFIED_INT,ARRAY_SIZE,BLOCK_COMMENT_STYLE,CONSTANT_COMPARISON \
	-f rdtset.c -f rdt.c -f rdt.h -f cpu.c -f cpu.h -f common.c -f common.h

CPPCHECK?=cppcheck
.PHONY: cppcheck
cppcheck:
	$(CPPCHECK) --enable=warning,portability,performance,unusedFunction,missingInclude \
	--std=c99 -I$(LIBDIR) --template=gcc --suppress=uninitvar \
	rdtset.c rdt.c rdt.h cpu.c cpu.h common.c common.h

# if target not clean then make dependencies
ifneq ($(MAKECMDGOALS),clean)
-include $(DEPFILES)
endif
