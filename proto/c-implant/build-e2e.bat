@echo off
x86_64-w64-mingw32-gcc -O2 -Wall -o cbeacon-e2e.exe beacon.c crypto_cng.c curve25519.c -lwinhttp -lbcrypt -lpsapi -liphlpapi -DC2_HOST=\"127.0.0.1\" -DC2_PORT=8001 -DBEACON_PATH=\"/collect\" -DSECRET_ID=\"YOUR_SECRET_ID\" -DSECRET_B64=\"YOUR_SECRET_B64\" -DINTERVAL=3 -DE2E_DEBUG
