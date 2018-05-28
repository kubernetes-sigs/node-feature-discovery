CC=gcc
LIBDIR=/usr/local/lib
INCDIR=../intel-cmt-cat/lib/

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

LDFLAGS=-L$(LIBDIR)
LDLIBS=-lpqos
CFLAGS=-I$(INCDIR)

BINARIES = mon-discovery \
		   mon-cmt-discovery \
		   mon-mbm-discovery \
		   l3-alloc-discovery \
		   l2-alloc-discovery \
		   mem-bandwidth-alloc-discovery

default:
	$(MAKE) all
all:
	$(CC) $(CFLAGS) -o mon-discovery monitoring-discovery.c $(LDFLAGS) $(LDLIBS)
	$(CC) $(CFLAGS) -o mon-cmt-discovery monitoring-cmt-discovery.c $(LDFLAGS) $(LDLIBS)
	$(CC) $(CFLAGS) -o mon-mbm-discovery monitoring-mbm-discovery.c $(LDFLAGS) $(LDLIBS)
	$(CC) $(CFLAGS) -o l3-alloc-discovery l3-allocation-discovery.c $(LDFLAGS) $(LDLIBS)
	$(CC) $(CFLAGS) -o l2-alloc-discovery l2-allocation-discovery.c $(LDFLAGS) $(LDLIBS)
	$(CC) $(CFLAGS) -o mem-bandwidth-alloc-discovery mem-bandwidth-allocation-discovery.c $(LDFLAGS) $(LDLIBS)

install:
	install -d $(BINDIR)
	install -t $(BINDIR) $(BINARIES)
