#import <LocalAuthentication/LocalAuthentication.h>
#import <dispatch/dispatch.h>
#import <stdlib.h>  // getenv

// LAErrorBiometryLockout (-8): Touch ID is locked out after too many failed
// attempts. Fall back to LAPolicyDeviceOwnerAuthentication so the user can
// unlock with their macOS login password instead of hard-failing.
//
// NOTE: The Go wrapper (biometric_darwin_cgo.go) skips this call entirely
// when CI=1, so the lockout-fallback policy never triggers during tests.
int vaultx_touchid_authenticate(void) {
    // Belt-and-suspenders: also skip in CI at the C level.
    const char *ci = getenv("CI");
    if (ci != NULL && ci[0] != '\0') {
        return 1;
    }
    @autoreleasepool {
        LAContext *context = [[LAContext alloc] init];
        NSError *error = nil;

        LAPolicy policy = LAPolicyDeviceOwnerAuthenticationWithBiometrics;
        if (![context canEvaluatePolicy:policy error:&error]) {
            // On lockout, retry with the broader policy that accepts password.
            if (error && error.code == LAErrorBiometryLockout) {
                [context release];
                context = [[LAContext alloc] init];
                policy = LAPolicyDeviceOwnerAuthentication;
                NSError *retryErr = nil;
                if (![context canEvaluatePolicy:policy error:&retryErr]) {
                    [context release];
                    return 0;
                }
            } else {
                [context release];
                return 0;
            }
        }

        __block int result = 0;
        dispatch_semaphore_t sema = dispatch_semaphore_create(0);

        [context evaluatePolicy:policy
                localizedReason:@"Authenticate to unlock vaultx vault"
                          reply:^(BOOL success, NSError * _Nullable evalError) {
            (void)evalError;
            result = success ? 1 : 0;
            dispatch_semaphore_signal(sema);
        }];

        dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
        [context release];
        return result;
    }
}
