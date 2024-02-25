#include <stdio.h>
#include <conio.h>
#include <thread>
#include <chrono>
#include <vector>
#include "parsec.h"
#include "export.h"

using namespace std::chrono_literals;
using namespace parsec_vdd;

bool running = false;
HANDLE vdd = NULL;
std::vector<int> displays;
int __cdecl init_virtual_display() {
    // Check driver status.
    DeviceStatus status = QueryDeviceStatus(&VDD_CLASS_GUID, VDD_HARDWARE_ID);
    if (status != DEVICE_OK)
    {
        printf("Parsec VDD device is not OK, got status %d.\n", status);
        return 1;
    }

    // Obtain device handle.
    running = true;
    vdd = OpenDeviceHandle(&VDD_ADAPTER_GUID);
    if (vdd == NULL || vdd == INVALID_HANDLE_VALUE) {
        printf("Failed to obtain the device handle.\n");
        return 1;
    }


    // Side thread for updating vdd.
    std::thread updater([&]() {
        while (running) {
            VddUpdate(vdd);
            std::this_thread::sleep_for(100ms);
        }
    });

    updater.detach();
    return 0;
}


int __cdecl add_virtual_display() {
	if (displays.size() < VDD_MAX_DISPLAYS) {
		int index = VddAddDisplay(vdd);
		displays.push_back(index);
		return 0;
	}

	return 1;
}

int __cdecl remove_virtual_display() {
	if (displays.size() > 0) {
		int index = displays.back();
		VddRemoveDisplay(vdd, index);
		displays.pop_back();
		return 0;
	}

	return 1;
}

int __cdecl deinit_virtual_display() {
    // Remove all before exiting.
    for (int index : displays) {
        VddRemoveDisplay(vdd, index);
    }

    // Close the device handle.
    CloseDeviceHandle(vdd);
	return 0;
}