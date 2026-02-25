package com.quantlink.trader.shm;

/**
 * SysV 共享内存操作异常。
 * <p>
 * 迁移自: hftbase/Ipc/include/sharedmemory.h
 * <p>
 * [C++差异] C++ 原代码抛出 std::string 异常（throw std::string(errorMsg)）；
 *           Java 使用标准 RuntimeException 子类，保持 unchecked 语义。
 * Ref: hftbase/Ipc/include/sharedmemory.h:77 — throw std::string(errorMsg);
 */
public class ShmException extends RuntimeException {

    public ShmException(String message) {
        super(message);
    }

    public ShmException(String message, Throwable cause) {
        super(message, cause);
    }
}
