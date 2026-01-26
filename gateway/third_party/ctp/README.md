# CTP SDK

**注意**: CTP SDK文件不包含在Git仓库中，需要自行下载安装。

## 📥 下载地址

- SimNow官网: https://www.simnow.com.cn/static/apiDownload.action
- 上期所官网: http://www.sfit.com.cn/DocumentDown/api_3/

## 📂 目录结构

安装完成后应包含以下文件：

```
ctp/
├── include/
│   ├── ThostFtdcMdApi.h
│   ├── ThostFtdcTraderApi.h
│   ├── ThostFtdcUserApiStruct.h
│   └── ThostFtdcUserApiDataType.h
└── lib/
    ├── thostmduserapi_se.so      (Linux)
    ├── thosttraderapi_se.so      (Linux)
    ├── error.xml
    └── error.dtd
```

## 🚀 快速安装

### Linux/Mac

```bash
# 下载CTP API v6.7.11或更新版本
# 解压后执行：

cp /path/to/ctp/ThostFtdc*.h include/
cp /path/to/ctp/*.so lib/
cp /path/to/ctp/error.* lib/
```

### Mac开发环境

Mac用户推荐使用Docker方案，参考：
- `docs/Mac开发环境配置_Docker方案_2026-01-26-16_00.md`

## ✅ 验证安装

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
