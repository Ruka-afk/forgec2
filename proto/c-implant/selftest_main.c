#include <stdio.h>
#include <windows.h>
#include "curve25519.h"
#include "crypto_cng.h"
int main(void) {
    DWORD len = 0;
    BYTE *d;
    fprintf(stderr, "A\n");
    d = b64dec("n88jGZSgdLfgpQS/C0PL4uRC+gcsnWeR+wwOq6HEX+I=", &len);
    fprintf(stderr, "B d=%p len=%lu\n", (void *)d, (unsigned long)len);
    printf("selftest=%d secretd=%p secretn=%lu\n", x25519_selftest(), (void *)d, (unsigned long)len);
    fflush(stdout);
    return 0;
}
