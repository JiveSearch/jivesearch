
<p align="center">
  <a href="https://github.com/adamfaliq42/jivesearch/edit/master/README.md">
    <img alt="jive-search logo" src="frontend/static/icons/logo.png">
  </a>
</p>

<br>


<p align="center">
Jive Search 是不跟踪你的开源搜索引擎. 页面根据他们的upvotes排名. 现在私下搜索 : https://www.jivesearch.com
</p>

<br>

<p align="center">
   <a href="https://github.com/jivesearch/jivesearch"><img src="https://img.shields.io/badge/go-1.10.2-blue.svg"></a>
   <a href="https://travis-ci.org/jivesearch/jivesearch"><img src="https://travis-ci.org/jivesearch/jivesearch.svg?branch=master"></a>
  <a href="https://github.com/jivesearch/jivesearch/blob/master/LICENSE"><img src="https://img.shields.io/badge/license-Apache-brightgreen.svg"></a>
</p>

<br>

  
## 🚩目录

- [浏览器支持](#browser-support)
- [快速开始](#quick-start)
- [安装I](#installation)
- [状态](#status)
- [路线图](#roadmap)
- [错误和功能请求](#bugs-and-feature-requests)
- [文档](#documentation)
- [贡献](#contributing)
- [社区](#community)
- [版权和许可](#copyright-and-license)


<br>

## 🌏 浏览器支持
| <img src="https://user-images.githubusercontent.com/1215767/34348387-a2e64588-ea4d-11e7-8267-a43365103afe.png" alt="Chrome" width="16px" height="16px" /> Chrome | <img src="https://user-images.githubusercontent.com/1215767/34348590-250b3ca2-ea4f-11e7-9efb-da953359321f.png" alt="IE" width="16px" height="16px" /> Internet Explorer | <img src="https://user-images.githubusercontent.com/1215767/34348380-93e77ae8-ea4d-11e7-8696-9a989ddbbbf5.png" alt="Edge" width="16px" height="16px" /> Edge | <img src="https://user-images.githubusercontent.com/1215767/34348394-a981f892-ea4d-11e7-9156-d128d58386b9.png" alt="Safari" width="16px" height="16px" /> Safari | <img src="https://user-images.githubusercontent.com/1215767/34348383-9e7ed492-ea4d-11e7-910c-03b39d52f496.png" alt="Firefox" width="16px" height="16px" /> Firefox |
| :---------: | :---------: | :---------: | :---------: | :---------: |
| Yes | 10+ | Yes | Yes | Yes |

<br>

## 🐾 快速开始
1. 转到Jive Search的[主页](https://www.jivesearch.com).
2.	开始搜索.
3.	Upvote或downvote页面!

<br>

## 💾 安装

1. 在[这里](https://golang.org/dl/)下载Go.
2. 在[这里](https://github.com/golang/go/wiki/SettingGOPATH)设置你的GOPATH
3. 安装Jive Search

```bash
$ go get -u github.com/jivesearch/jivesearch
```
4.	运行测试

```bash
cd $HOME/go/src/github.com/jivesearch/jivesearch && go test -cover -race ./...
```

##### Crawler
需要Elasticsearch和Redis..

```bash
$ cd $GOPATH/src/github.com/jivesearch/jivesearch/search/crawler && go run ./cmd/crawler.go --workers=75 --time=5m --debug=true
```
  
##### Frontend
需要Elasticsearch和PostgreSQL.

```bash
$ cd $GOPATH/src/github.com/jivesearch/jivesearch/frontend && go run ./cmd/frontend.go --debug=true
```

##### Wikipedia Dump File
需要PostgreSQL.

```bash
$ cd $GOPATH/src/github.com/jivesearch/jivesearch/instant/wikipedia/cmd/dumper && go run dumper.go --workers=3 --dir=/path/to/wiki/files --text=true --data=true --truncate=400
```
<br>

## 🚀 **文档* 
### 我们的目标是创建尊重您隐私的搜索引擎，并提供出色的搜索结果，即时答案，地图，图片搜索，新闻等。 

有标记的项目表明该类别已取得进展。 每个领域还有很多事情要做。 欢迎提出建议!. 

- [x] 隐私
- [x] ！邦斯
- [x] 核心搜索结果和分布式爬网程序
    - [x] 语言和地区
    - [ ] 高级搜索（精确短语，狗或猫， - 邮件，网站/域搜索等）
    - [ ] 文件类型
    - [ ] 安全搜索      
    - [ ] 时间搜索（过去的年/月/日/小时等）
- [x] 自动完成
- [ ] 短语建议者（又名“你是否意思？”）
- [x] 即时答案
    - [x] 诞生石，camelcase，人物，投掷硬币，频率，POTUS，素数，随机数，反转数，统计数据，用户代理等。
    - [x] 唱片/音乐专辑
    - [x] 基于JavaScript的答案
        - [x] 基本计算器
            - [ ] 抵押贷款，金融和其他计算器
        - [x] CSS / JavaScript / JSON /等缩小和美化
        - [x] 转换器（米到英尺，MB到GB等）
    - [x] 包裹追踪（UPS，FedEx，USPS等）
    - [x] 堆栈溢出
    - [x] 股票行情和图表
    - [x] 天气
    - [x] 维基百科摘要
    - [x] 维基数据答案（多高，生日等）
    - [x] Wikiquote
    - [x] 维基词典
    - [x] 更多即时答案（包括来自第三方API）
    - [x] 翻译触发词和其他语言的答案？
- [x] 地图
- [x] 图像搜索
- [x] 视频搜索
- [x] 航班信息和状态
- [x] 新闻
- [x] 购物
- [x] 自定义主题
- [ ] 新名称/标志
- [ ] 文档
    - [ ] 翻译成中文，法文和其他语言
    - [ ] Jive Search如何工作？(link to /about page)
    - [ ] 我们如何照顾隐私？
    - [ ] 测试和基准

<br>

## 📙 贡献
Jive Search的文档托管在这里的GoDoc页面上.

<br>

## 💬 特约
想贡献？ 大！
搜索现有和已结束的问题。 如果您的问题或想法尚未解决，请开启一个新问题.

<br>

## 📜 版权和许可
代码和文档版权所有2018 Jivesearch作者。 根据Apache许可证发布的代码和文档.