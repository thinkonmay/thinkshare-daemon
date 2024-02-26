#include <Windows.h>
#include <conio.h>
#include <stdio.h>
typedef int (*FUNC)	();
typedef int (*FUNC2)	(int width, int height, char* byte, int* size);

static FUNC _init_virtual_display;
static FUNC _deinit_virtual_display;
static FUNC _remove_virtual_display;
static FUNC2 _add_virtual_display;

int
initlibrary() {
	HMODULE hModule 	= LoadLibraryA(".\\libdisplay.dll");
	_init_virtual_display 	= (FUNC)	GetProcAddress( hModule,"init_virtual_display");
	_deinit_virtual_display = (FUNC)	GetProcAddress( hModule,"deinit_virtual_display");
	_remove_virtual_display	= (FUNC)	GetProcAddress( hModule,"remove_virtual_display");
	_add_virtual_display 	= (FUNC2)	GetProcAddress( hModule,"add_virtual_display");

    if (_init_virtual_display == 0 ||
        _deinit_virtual_display == 0 ||
        _add_virtual_display == 0 ||
        _remove_virtual_display == 0) 
        return 1;


	return 0;
}


int main() {
    if(initlibrary())
        printf("failed to load libdisplay.dll\n");


    _init_virtual_display();

    char name[1024] = {0};
    int size =0;
    _add_virtual_display(3840,2160,name,&size);
    printf("%s\n",name);
    _getch();
}