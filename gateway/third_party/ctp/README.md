# CTP SDK

**注意**: CTP SDK文件不包含在Git仓库中，需要自行下载安装。

## 📥 下载地址

- SimNow官网: https://www.simnow.com.cn/static/apiDownload.action
- 上期所官网: http://www.sfit.com.cn/DocumentDown/api_3/

## 📂 目录结构

### MacOS版本（推荐）

```
ctp/
├── include/
│   ├── ThostFtdcMdApi.h
│   ├── ThostFtdcTraderApi.h
│   ├── ThostFtdcUserApiStruct.h
│   └── ThostFtdcUserApiDataType.h
├── thostmduserapi_se.framework/    (MacOS Framework)
└── thosttraderapi_se.framework/    (MacOS Framework)
```

### Linux版本

```
ctp/
├── include/
│   └── (同上)
└── lib/
    ├── thostmduserapi_se.so
    ├── thosttraderapi_se.so
    ├── error.xml
    └── error.dtd
```

## 🚀 快速安装

### MacOS（推荐）

```bash
# 从SimNow下载MacOS版本
# 在下载页面选择: MacOS -> 看穿式监管生产版

# 解压后复制framework
cp -R /path/to/API/thostmduserapi_se.framework ./
cp -R /path/to/API/thosttraderapi_se.framework ./

# 复制头文件到include目录（方便CMake查找）
cp thostmduserapi_se.framework/Headers/*.h include/
cp thosttraderapi_se.framework/Headers/ThostFtdcTraderApi.h include/
```

**架构支持**:
- ✅ Apple Silicon (M1/M2/M3) - arm64
- ✅ Intel Mac - x86_64

### Linux

```bash
# 下载Linux版本
cp /path/to/ctp/ThostFtdc*.h include/
cp /path/to/ctp/*.so lib/
cp /path/to/ctp/error.* lib/
```

## ✅ 验证安装

### MacOS
```bash
ls include/                      # 应该看到4个头文件
ls *.framework                   # 应该看到2个framework
file thostmduserapi_se.framework/thostmduserapi_se  # 检查架构
```

### Linux
```bash
ls include/  # 应该看到4个头文件
ls lib/      # 应该看到2个.so文件和2个error文件
```

## 📝 版本信息

- **推荐版本**: v6.7.11或更新
- **平台**: Linux x86-64
- **类型**: 看穿式监管版本（非商密）

## 🔗 相关文档

- [任务#1 CTP行情接入实施指南](../../../docs/任务1_CTP行情接入实施指南_2026-01-26-15_40.md)
- [Mac开发环境配置](../../../docs/Mac开发环境配置_Docker方案_2026-01-26-16_00.md)
