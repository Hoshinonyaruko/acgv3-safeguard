<p align="center">
    <img src="pic/1.jpg" alt="acgv3-safeguard Logo" width="200">
</p>

<h1 align="center">acgv3-safeguard</h1>
<p align="center">异次元v3项目安全守护</p>

<p align="center">
    欢迎提交 PR 以改进本项目！
</p>

---

## 项目介绍

`acgv3-safeguard` 为异世界商城 (acgv3) 带来额外的安全性。通过保护 MySQL 数据库中的敏感表及支付与插件配置文件，防止未授权的修改或恶意操作，从而提升整体安全性。

## 功能

- **管理员列表保护**  
  自动锁定管理员表 (`acg_manage`) 中仅保留 ID 为 `1` 的管理员，删除其他多余记录。
  
- **支付方式保护**  
  保护支付表 (`acg_pay`)，防止新增、删除或修改支付方式，任何改动都会被自动还原。

- **支付插件配置保护**  
  按照配置文件的设定，从源目录同步支付插件和配置文件到目标目录，确保插件配置文件的完整性。

- **插件配置保护**  
  同步插件配置文件，防止被恶意替换或修改。

---

## 使用前提示 ⚠️

1. **管理员锁定**  
   程序会自动将管理员表 (`acg_manage`) 锁定为仅保留 1 个管理员（ID 为 1）。请谨慎使用此功能，并做好数据备份。

2. **支付通道设置锁定**  
   程序启动后，支付通道设置将被锁死，任何修改都将自动被还原。

3. **目录同步**  
   需要用户手动从配置的源目录复制支付与插件配置文件到目标目录，配置示例如下：
   ```yaml
   mysql:
       address: 127.0.0.1:3306
       username: root
       password: 
   paths:
       pay_override_source: "C:\\Users\\xxxxxx\\Desktop\\acgv3\\Pay"
       pay_override_target: "C:\\xxxxxx\\htdocs\\app\\Pay"
       plugin_override_source: "C:\\Users\\xxxxxx\\Desktop\\acgv3\\Plugin"
       plugin_override_target: "C:\\xxxxxx\\htdocs\\app\\Plugin"
   protection:
       admin_table: true
       payment_table: true


    注意: Windows 用户请在路径中使用双写反斜杠 (\\)。
