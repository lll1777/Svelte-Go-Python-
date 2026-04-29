# 智能农业病虫害诊断与农事服务工单平台

面向种植户与农技专家的远程诊断平台，实现作物拍照诊断、AI自动识别、专家分配、在线问诊、防治效果反馈等完整功能。

## 技术栈

- **前端**: SvelteKit + JavaScript
- **后端**: Go + Gin + GORM + WebSocket
- **AI服务**: Python + Flask + Pillow
- **数据库**: SQLite (可扩展为PostgreSQL/MySQL)
- **离线存储**: IndexedDB (idb库)

## 核心功能

### 农户端功能
1. **拍照诊断** - 上传作物照片，AI自动识别病虫害
2. **离线支持** - 无网络环境下创建工单，联网后自动同步
3. **工单管理** - 查看所有诊断工单状态
4. **在线问诊** - 与农技专家实时聊天交流
5. **处方查看** - 查看专家开具的防治方案
6. **效果反馈** - 提交防治效果评价和评分

### 专家端功能
1. **专家工作台** - 管理待诊断和进行中的工单
2. **工单分配** - 基于地理位置自动分配最近专家
3. **开具处方** - 为农户提供专业的防治方案
4. **在线问诊** - 与农户实时沟通
5. **服务考核** - 基于农户反馈的评分系统

### AI服务功能
1. **图像识别** - 基于图片哈希和特征分析识别病虫害
2. **方案生成** - 根据病害类型自动生成防治方案
3. **配伍禁忌** - 检查农药配方的安全性
4. **相似病例** - 推荐历史相似病例供参考

### 图片与问诊单关联机制
- 每张图片生成唯一的感知哈希(perceptual hash)
- 哈希值与工单ID关联存储
- 支持通过图片哈希查询关联的问诊单
- 离线模式下本地存储关联关系

## 项目结构

```
Svelte-Go-Python-/
├── frontend/              # Svelte前端应用
│   ├── src/
│   │   ├── routes/        # 页面路由
│   │   │   ├── login/     # 登录注册页
│   │   │   ├── diagnose/  # 拍照诊断页
│   │   │   ├── work-orders/ # 工单列表和详情
│   │   │   ├── expert/workbench/ # 专家工作台
│   │   │   ├── +layout.svelte   # 布局组件
│   │   │   └── +page.svelte     # 首页
│   │   ├── stores/        # 状态管理
│   │   │   ├── auth.js    # 用户认证状态
│   │   │   └── workOrders.js # 工单状态
│   │   ├── lib/
│   │   │   ├── api/       # API客户端
│   │   │   ├── services/  # 服务层(WebSocket等)
│   │   │   └── utils/     # 工具函数(离线存储)
│   ├── package.json
│   ├── svelte.config.js
│   └── vite.config.js
│
├── backend/               # Go后端服务
│   ├── config/            # 配置管理
│   ├── controllers/       # API控制器
│   ├── database/          # 数据库连接和初始化
│   ├── middleware/        # 中间件(JWT认证)
│   ├── models/            # 数据模型
│   ├── routes/            # 路由配置
│   ├── services/          # 业务逻辑层
│   ├── go.mod
│   └── main.go
│
└── ai_service/            # Python AI服务
    ├── app.py             # Flask主应用
    ├── config.py          # 配置
    ├── disease_database.py # 病虫害数据库
    ├── image_analyzer.py  # 图像分析器
    ├── prescription_checker.py # 处方检查器
    ├── case_recommender.py    # 病例推荐
    └── requirements.txt
```

## 快速开始

### 环境要求
- Go 1.21+
- Python 3.10+
- Node.js 18+

### 1. 后端服务 (Go)

```bash
cd backend

# 下载依赖
go mod download

# 运行服务 (默认端口8080)
go run main.go
```

### 2. AI服务 (Python)

```bash
cd ai_service

# 创建虚拟环境
python -m venv venv

# 激活虚拟环境
# Windows:
venv\Scripts\activate
# Linux/Mac:
source venv/bin/activate

# 安装依赖
pip install -r requirements.txt

# 运行服务 (默认端口5000)
python app.py
```

### 3. 前端应用 (Svelte)

```bash
cd frontend

# 安装依赖
npm install

# 开发模式运行 (默认端口5173)
npm run dev
```

## API接口

### 认证接口
- `POST /api/auth/register` - 用户注册
- `POST /api/auth/login` - 用户登录

### 工单接口
- `POST /api/work-orders` - 创建工单
- `POST /api/work-orders/upload-diagnose` - 上传图片并诊断
- `GET /api/work-orders/my` - 获取我的工单
- `GET /api/work-orders/:id` - 获取工单详情
- `PATCH /api/work-orders/:id/status` - 更新工单状态
- `POST /api/work-orders/:id/prescription` - 开具处方
- `POST /api/work-orders/:id/feedback` - 提交反馈
- `GET /api/work-orders/check-image-association` - 检查图片关联

### AI服务接口
- `POST /api/diagnose` - 图片诊断
- `POST /api/check-prescription` - 处方兼容性检查
- `POST /api/similar-cases` - 获取相似病例
- `POST /api/generate-plan` - 生成治疗方案

### WebSocket接口
- `GET /api/ws` - WebSocket连接
- 支持消息类型: `join`, `leave`, `message`, `typing`, `ping`

## 数据模型

### 工单状态流转
```
pending(待诊断) 
  → diagnosing(AI诊断中) 
  → assigned(已分配专家) 
  → consulting(问诊中) 
  → prescribed(已开处方) 
  → confirmed(用户确认) 
  → closed(已关闭)
```

### 核心实体
- **User** - 用户(农户/专家/管理员)
- **WorkOrder** - 工单
- **DiagnosisResult** - AI诊断结果
- **Prescription** - 专家处方
- **Feedback** - 用户反馈
- **WorkOrderImage** - 工单图片(含哈希)
- **Message** - 聊天消息

## 离线功能

前端支持以下离线功能:
1. 创建工单
2. 查看已缓存的工单
3. 查看已缓存的消息
4. 本地存储图片与工单关联

联网后自动:
1. 同步离线创建的工单
2. 同步离线发送的消息
3. 更新本地缓存数据

## 演示账户

系统初始化时会创建以下演示账户:

| 用户名 | 密码 | 角色 | 说明 |
|--------|------|------|------|
| farmer1 | password123 | 种植户 | 普通农户账户 |
| expert1 | password123 | 农技专家 | 高级农艺师 |
| expert2 | password123 | 农技专家 | 农业技术推广研究员 |

## 配置说明

### 后端环境变量
```
SERVER_PORT=8080
DATABASE_URL=sqlite3:./agriculture.db
JWT_SECRET=your-secret-key
PYTHON_SERVICE_URL=http://localhost:5000
```

### AI服务环境变量
```
PORT=5000
DEBUG=True
```

## 扩展建议

1. **数据库**: 生产环境建议使用PostgreSQL或MySQL
2. **图像识别**: 集成真实的深度学习模型(如TensorFlow/PyTorch)
3. **对象存储**: 使用S3/OSS存储图片
4. **推送通知**: 集成消息推送服务
5. **地理位置服务**: 集成地图API
6. **支付系统**: 添加服务计费功能

## 许可证

MIT License
