CXX ?= g++
CXXFLAGS := -std=c++11 -O0

# hftbase headers
HFTBASE_DIR ?= $(abspath ../../../hftbase)
IPC_INCLUDE := $(HFTBASE_DIR)/Ipc/include
COMMON_INCLUDE := $(HFTBASE_DIR)/CommonUtils/include

INCLUDES := -I$(COMMON_INCLUDE) -I$(IPC_INCLUDE)

.PHONY: all clean

all: offset_check

offset_check: offset_check.cpp
	$(CXX) $(CXXFLAGS) $(INCLUDES) -o $@ $<

clean:
	rm -f offset_check
