extern "C" {
__declspec(dllexport) int __cdecl init_virtual_display();
__declspec(dllexport) int __cdecl deinit_virtual_display();
__declspec(dllexport) int __cdecl add_virtual_display(int width, int height, char* byte, int* size);
__declspec(dllexport) int __cdecl remove_virtual_display(int);
}