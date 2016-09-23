#!/usr/bin/env Rscript
library(ggplot2)

argv <- commandArgs(T)
inFile <- argv[1]
outFile <- argv[2]
tab = read.table(inFile)
dat <- data.frame(Discovery = tab[,1], Time = tab[,2])
bplot <- ggplot(dat, aes(x=Discovery, y=Time, fill=Discovery)) + geom_boxplot() + guides(fill=FALSE) + ggtitle("Performance Comparison With and Without Discovery Enabled") + xlab("") + ylab("Time in Seconds") + expand_limits(y=0)
ggsave(outFile, device="pdf")
