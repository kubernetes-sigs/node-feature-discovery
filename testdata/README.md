Some of the test data in this folder contains colon characters which causes "go get" commands to fail with "invalid char ':'".
The empty go.mod file is a workaround to prevent this error. This effectively makes this folder its own go module, so it will be
ignored when "go get" is executed.
