#include <zec.h>

// This file exists beacause of
// https://github.com/golang/go/issues/11263

void cgo_rust_task_callback_bridge_zec(RustTaskCallback cb, const void * taskData, int8_t status) {
  cb(taskData, status);
}