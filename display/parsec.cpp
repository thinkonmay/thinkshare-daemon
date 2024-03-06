#include <stdio.h>
#include <conio.h>
#include <thread>
#include <string>
#include <chrono>
#include <vector>
#include "parsec.h"
#include "export.h"

using namespace std::chrono_literals;
using namespace parsec_vdd;

bool running = false;
HANDLE vdd = NULL;
auto displays = new std::vector<int>();
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


int __cdecl add_virtual_display(int width, int height, char* byte, int* size) {
	if (displays->size() >= VDD_MAX_DISPLAYS) {
		return 1;
	}

	auto pre = Displays();
	int index = VddAddDisplay(vdd);
	displays->push_back(index);
	std::this_thread::sleep_for(5s);
	auto after = Displays();

    bool failed = true;
    for (auto a : after) {
        if (strcmp(VDD_ADAPTER_NAME, a.DeviceString) != 0)
            continue;

		bool n = true;
		for (auto p : pre) {
			if (strcmp(a.DeviceName, p.DeviceName) == 0)
				n = false;
		}

		if (n) {
			SetResolution(a.DeviceName,width,height,240);
            memcpy(byte,a.DeviceName,strlen(a.DeviceName));
            *size = strlen(a.DeviceName);
            failed = false;
        } 
    }

	return failed ? index : -1;

}

int __cdecl remove_virtual_display(int index) {
    auto replace = new std::vector<int>();
    for (size_t i = 0; i < displays->size(); i++) {
        if (displays->at(i) == index) 
		    VddRemoveDisplay(vdd, index);
        else 
            replace->push_back(displays->at(i));
    }
    

    displays = replace;
}

int __cdecl deinit_virtual_display() {
    // Remove all before exiting.
    for (int index : *displays) {
        VddRemoveDisplay(vdd, index);
    }

    // Close the device handle.
    CloseDeviceHandle(vdd);
    running = false;
	return 0;
}


