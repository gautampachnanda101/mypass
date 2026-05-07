//go:build darwin
// +build darwin

package keychain

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Security -framework Foundation
#import <Security/Security.h>
#import <Foundation/Foundation.h>

// Store a password in the macOS Keychain
int keychain_store(const char *service, const char *account, const char *password) {
    NSString *serviceStr = [NSString stringWithUTF8String:service];
    NSString *accountStr = [NSString stringWithUTF8String:account];
    NSString *passwordStr = [NSString stringWithUTF8String:password];
    NSData *passwordData = [passwordStr dataUsingEncoding:NSUTF8StringEncoding];

    // Delete existing item first
    NSDictionary *deleteQuery = @{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: serviceStr,
        (__bridge id)kSecAttrAccount: accountStr
    };
    SecItemDelete((__bridge CFDictionaryRef)deleteQuery);

    // Add new item
    NSDictionary *addQuery = @{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: serviceStr,
        (__bridge id)kSecAttrAccount: accountStr,
        (__bridge id)kSecValueData: passwordData,
        (__bridge id)kSecAttrAccessible: (__bridge id)kSecAttrAccessibleWhenUnlockedThisDeviceOnly
    };

    OSStatus status = SecItemAdd((__bridge CFDictionaryRef)addQuery, NULL);
    return (int)status;
}

// Load a password from the macOS Keychain
// Returns NULL if not found
char* keychain_load(const char *service, const char *account) {
    NSString *serviceStr = [NSString stringWithUTF8String:service];
    NSString *accountStr = [NSString stringWithUTF8String:account];

    NSDictionary *query = @{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: serviceStr,
        (__bridge id)kSecAttrAccount: accountStr,
        (__bridge id)kSecReturnData: @YES,
        (__bridge id)kSecMatchLimit: (__bridge id)kSecMatchLimitOne
    };

    CFTypeRef result = NULL;
    OSStatus status = SecItemCopyMatching((__bridge CFDictionaryRef)query, &result);

    if (status != errSecSuccess || result == NULL) {
        return NULL;
    }

NSData *passwordData = (__bridge NSData *)result;
	NSString *password = [[NSString alloc] initWithData:passwordData encoding:NSUTF8StringEncoding];
	char *result_str = strdup([password UTF8String]);
	CFRelease(result);
	return result_str;
}

// Delete a password from the macOS Keychain
int keychain_delete(const char *service, const char *account) {
    NSString *serviceStr = [NSString stringWithUTF8String:service];
    NSString *accountStr = [NSString stringWithUTF8String:account];

    NSDictionary *query = @{
        (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
        (__bridge id)kSecAttrService: serviceStr,
        (__bridge id)kSecAttrAccount: accountStr
    };

    OSStatus status = SecItemDelete((__bridge CFDictionaryRef)query);
    return (int)status;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func storeImpl(service, account, secret string) error {
	cService := C.CString(service)
	cAccount := C.CString(account)
	cSecret := C.CString(secret)
	defer C.free(unsafe.Pointer(cService))
	defer C.free(unsafe.Pointer(cAccount))
	defer C.free(unsafe.Pointer(cSecret))

	status := C.keychain_store(cService, cAccount, cSecret)
	if status != 0 {
		return fmt.Errorf("keychain store failed: OSStatus %d", status)
	}
	return nil
}

func loadImpl(service, account string) (string, error) {
	cService := C.CString(service)
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cService))
	defer C.free(unsafe.Pointer(cAccount))

	cSecret := C.keychain_load(cService, cAccount)
	if cSecret == nil {
		return "", fmt.Errorf("not found")
	}
	defer C.free(unsafe.Pointer(cSecret))

	return C.GoString(cSecret), nil
}

func deleteImpl(service, account string) error {
	cService := C.CString(service)
	cAccount := C.CString(account)
	defer C.free(unsafe.Pointer(cService))
	defer C.free(unsafe.Pointer(cAccount))

	status := C.keychain_delete(cService, cAccount)
	if status != 0 && status != -25300 { // -25300 = errSecItemNotFound
		return fmt.Errorf("keychain delete failed: OSStatus %d", status)
	}
	return nil
}
