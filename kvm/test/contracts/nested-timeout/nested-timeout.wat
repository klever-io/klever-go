(module
 (type $executeOnDestContext (func (param i64 i32 i32 i32 i32 i32 i32 i32) (result i32)))
 (import "env" "executeOnDestContext" (func $executeOnDestContext (type $executeOnDestContext)))
 (memory (export "memory") 1)
 (data (i32.const 1024) "\00\00\00\00\00\00\00\00\0f\0fparentSC..............")
 (data (i32.const 1056) "\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00\00")
 (data (i32.const 1088) "child")
 (func (export "nestedForever")
  i64.const 1000000000
  i32.const 1024
  i32.const 1056
  i32.const 1088
  i32.const 5
  i32.const 0
  i32.const 0
  i32.const 0
  call $executeOnDestContext
  drop)
 (func (export "child")
  (loop $again
   br $again)))
