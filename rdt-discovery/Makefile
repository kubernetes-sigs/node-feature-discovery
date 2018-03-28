CC=gcc
LIBDIR=/usr/local/lib
INCDIR=../intel-cmt-cat/lib/

LDFLAGS=-L$(LIBDIR)
LDLIBS=-lpqos
CFLAGS=-I$(INCDIR)

default:
	$(MAKE) all
all:
	$(CC) $(CFLAGS) -o mon-discovery monitoring-discovery.c $(LDFLAGS) $(LDLIBS)
	$(CC) $(CFLAGS) -o l3-alloc-discovery l3-allocation-discovery.c $(LDFLAGS) $(LDLIBS)
	$(CC) $(CFLAGS) -o l2-alloc-discovery l2-allocation-discovery.c $(LDFLAGS) $(LDLIBS)
