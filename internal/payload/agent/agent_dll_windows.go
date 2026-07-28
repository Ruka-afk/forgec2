//go:build windows

package main

import "C"

//export DllInstall
func DllInstall() {
	go main()
}

//export Start
func Start() {
	go main()
}

//export DllRegisterServer
func DllRegisterServer() {
	go main()
}
