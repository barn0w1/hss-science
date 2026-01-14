# HSS Science Platform

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

> **HSS Science のための統合クラウド基盤**

[ [English](./README.md) | **日本語** ]

## 概要

**hss-science** は、HSS Science のコラボレーションワークフローを支えるために設計されたプライベートクラウドプラットフォームです。

複数のツールを使い分けることは柔軟性をもたらしますが、ツール間の連携不足はしばしばボトルネックになります。ファイルの保存、処理の自動化、計算リソースの確保—これらを異なるサービスで管理すると、認証の重複、データの分散、遅延が避けられません。

本プラットフォームは、認証・ストレージ・計算を統合することで、**パフォーマンス**、**データ主権**、**シンプルな開発体験**を実現します。マイクロサービスアーキテクチャにより、新しい機能を段階的に追加・改善することで、ワークフローの拡張に対応します。

## サービス構成

本システムはgRPCマイクロサービスアーキテクチャで構築されており、REST API（gRPC-Gateway経由）を提供します。

| サービス | エンドポイント | 概要 |
| :--- | :--- | :--- |
| **Auth** | `accounts.hss-science.org` | **認証・セッション管理**。<br>Discord OAuth による一元的な認証基盤。JWT 発行およびセッション管理を担当します。 |
| **Drive** | `drive.hss-science.org` | **統合ストレージ**。<br>Cloudflare R2 をバックエンドとする Content Addressable Storage。ファイルの不変性と効率的な重複排除を実現します。 |
| **Compute** | `compute.hss-science.org` | **動的計算基盤**。<br>GPU / CPU インスタンスをオーケストレーション。Blender レンダリング、データ処理、ファームウェアコンパイルなどの高負荷ワークロードを実行します。 |

## アーキテクチャ

### 設計原則

**スキーマ駆動開発**  
API定義はすべて `proto/` に集約。Protocol Buffersから自動的にGo（サーバー）とTypeScript（クライアント）のコードを生成し、型安全性を保証します。

**サービス隔離**  
各サービスはドメイン境界で完全に分離。サービス間通信は gRPC のみです。

**明示的な設定**  
依存関係や環境設定をコード上に明示的に記述。暗黙的な動作を排除し、可読性を維持します。

### 技術スタック

- **Backend**: Go 1.25+, gRPC, sqlx
- **Frontend**: TypeScript, React, pnpm workspaces
- **Infrastructure**: PostgreSQL, Redis, Cloudflare R2, Docker

## 開発を始める

### 前提条件

- Go 1.25+
- Docker & Docker Compose
- `buf`（Protocol Buffersコード生成ツール）

### クイックスタート

**1. Proto定義からコードを生成**
```bash
make gen
```

**2. ローカル開発環境を起動**
```bash
make infra-up    # PostgreSQL, Redis を起動
make infra-down  # 停止
```

**3. バックエンドサービスを実行**
```bash
make server-run
```

詳細は [Makefile](./Makefile) を参照してください。

## ライセンス

GNU Affero General Public License v3.0 (AGPL-3.0)  
詳細は [LICENSE](./LICENSE) を参照してください。