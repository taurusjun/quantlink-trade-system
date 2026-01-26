# 配置文件说明

## 🔐 CTP账号配置

### 首次配置步骤

1. **复制示例文件**
   ```bash
   cd config
   cp ctp_md.secret.yaml.example ctp_md.secret.yaml
   ```

2. **编辑密码文件**
   ```bash
   vim ctp_md.secret.yaml
   # 或
   code ctp_md.secret.yaml
   ```

3. **填写您的SimNow账号**
   ```yaml
   credentials:
     user_id: "YOUR_USER_ID"      # 替换为您的用户ID
     password: "YOUR_PASSWORD"    # 替换为您的密码
   ```

4. **验证配置**
   ```bash
   cd ..
   ./test_ctp_account.sh
   ```

### 文件说明

| 文件 | 说明 | 提交到Git？ |
|------|------|------------|
| `ctp_md.yaml` | 主配置（不含密码） | ✅ 是 |
| `ctp_md.secret.yaml` | **密码文件**（真实账号） | ❌ **否** |
| `ctp_md.secret.yaml.example` | 密码文件示例 | ✅ 是 |
| `.gitignore` | Git忽略规则 | ✅ 是 |

### ⚠️ 安全提醒

- ❌ **永远不要**提交 `ctp_md.secret.yaml` 到Git
- ❌ **永远不要**在代码中硬编码密码
- ✅ `ctp_md.secret.yaml` 已被 `.gitignore` 保护
- ✅ 密码文件仅存储在本地

### 多环境配置

如需配置多个环境：

```bash
# 开发环境
cp ctp_md.secret.yaml.example ctp_md.secret.dev.yaml

# 生产环境
cp ctp_md.secret.yaml.example ctp_md.secret.prod.yaml
```

程序启动时可指定：
```bash
./ctp_md_gateway -secret config/ctp_md.secret.dev.yaml
```

---

## 📝 其他配置文件

### trader.yaml
Golang交易系统主配置文件。

### trader.test.yaml
测试环境配置。

---

**最后更新**: 2026-01-26
