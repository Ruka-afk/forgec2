@echo off
x86_64-w64-mingw32-gcc -O2 -Wall -o cbeacon-test.exe beacon.c crypto_cng.c curve25519.c -lwinhttp -lbcrypt -lpsapi -liphlpapi -DC2_HOST=\"127.0.0.1\" -DC2_PORT=8000 -DSECRET_ID=\"x\" -DSECRET_B64=\"eA==\"
