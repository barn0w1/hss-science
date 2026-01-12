# HSS Science Platform

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

[ **English** | [日本語](#-日本語-japanese) ]

The unified distributed platform for the HSS Science community.
Designed to maintain structural integrity and minimize complexity through strict architectural discipline.

## Philosophy

Our goal is to build a robust, scalable system by reducing entropy in software development.

- **Single Source of Truth**: A monorepo structure to ensure consistency across all services and infrastructure.
- **Do One Thing Well**: Strictly decoupled microservices (`apps`) sharing a standardized foundation (`pkg`).
- **Simplicity and Performance**: Powered by Go to enforce type safety and efficiency.

## Architecture

This repository hosts the entire ecosystem, orchestrated as a distributed system:

### Core Services
- **`apps/auth`**: Identity Provider (IdP) and SSO foundation based on JWT.
- **`apps/drive`**: Content Addressable Storage (CAS) for immutable data management.
- **`apps/render`**: Controller for the distributed render farm, orchestrating GPU instances.

### Infrastructure
- **`pkg`**: Shared standard libraries (Logger, Config, Audit).
- **`proto`**: gRPC definitions serving as the immutable contract between services.

## Status

**Under active development.**

---

## 🇯🇵 日本語 (Japanese)

HSS Science コミュニティのための統合分散プラットフォームです。
厳格なアーキテクチャ規律を通じて複雑性を排除し、システムの整合性を保つよう設計されています。

### 設計思想

ソフトウェア開発におけるエントロピー（無秩序）の増大を抑制し、堅牢でスケーラブルなシステムを構築します。

- **Single Source of Truth**: モノレポ構成により、全サービスとインフラの一貫性を保証します。
- **Do One Thing Well**: マイクロサービス（`apps`）は単一の責務を持ち、共通基盤（`pkg`）を利用します。
- **Simplicity and Performance**: Go言語を採用し、型安全性と高いパフォーマンスを実現します。

### アーキテクチャ

このリポジトリは、分散システムとして動作するエコシステム全体を管理します。

#### コアサービス
- **`apps/auth`**: 認証基盤 (IdP)。JWTベースのSSOを提供します。
- **`apps/drive`**: ストレージ基盤。CAS（Content Addressable Storage）によりデータの不変性を担保します。
- **`apps/render`**: 分散レンダーファームのコントローラー。GPUインスタンスのオーケストレーションを行います。

#### インフラストラクチャ
- **`pkg`**: 共通標準ライブラリ（ロガー、設定管理、監査ログなど）。
- **`proto`**: gRPC定義ファイル。サービス間の不変の契約（コントラクト）として機能します。

### ステータス

**開発中 (Pre-alpha)**