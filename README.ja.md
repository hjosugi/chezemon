# Chezemon

日本語 · [English](README.md)

[![CI](https://github.com/hjosugi/chezemon/actions/workflows/ci.yml/badge.svg)](https://github.com/hjosugi/chezemon/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-b8f36b.svg)](LICENSE)

Chezemonは、[chezmoi](https://www.chezmoi.io/)の状態を分かりやすくする
ローカルファーストのビジュアル管理画面です。remoteの履歴から、実際にホーム
ディレクトリへ置かれているファイルまでを一つの画面で表示し、次に何を確認
すべきか案内します。

現在のChezemonは意図的に読み取り専用です。ファイルを書き換える前に状態を
理解し、必要な変更を失わないことを優先しています。

## クイックスタート

必要なもの：

- Go 1.26以上
- 現在のユーザーで初期化済みのchezmoi
- chezmoi sourceをGit管理している場合はGit

[Releases](https://github.com/hjosugi/chezemon/releases)からバイナリを
ダウンロードし、検証して実行します。

```bash
sha256sum -c SHA256SUMS --ignore-missing
chmod +x chezemon-linux-amd64
./chezemon-linux-amd64 --open
```

ソースからビルドする場合：

```bash
git clone https://github.com/hjosugi/chezemon.git
cd chezemon
go run ./cmd/chezemon --open
```

`http://127.0.0.1:41273`のようなloopback URLが表示されます。ブラウザが
自動で開かない場合は、表示されたURLを直接開いてください。

Windows、macOS、Linuxに対応します。UIは単一のGoバイナリへ埋め込まれており、
Electron、Node.js、Python、OS固有のWebView runtimeは必要ありません。

### オプション

| フラグ | 既定値 | 説明 |
| --- | --- | --- |
| `--listen` | `127.0.0.1:0` | 待ち受けアドレス。loopback以外は終了コード2で拒否。 |
| `--open` | `false` | 表示したURLを既定のブラウザで開く。 |
| `--snapshot` | `false` | JSONスナップショットを1回標準出力へ出して終了。 |
| `--timeout` | `3m` | chezmoi処理1回あたりの上限時間。 |
| `--debug` | `false` | リクエストのパスをdebugレベルで記録。 |
| `--version` | `false` | ビルドのバージョン・コミット・プラットフォームを表示して終了。 |

## 何を解決するもの？

dotfilesの状態は、実際には次の4層に分かれています。

```text
remote/upstream -> Git/source -> rendered target -> live home
      履歴            宣言          このPCの理想状態       実際の状態
```

`git status`が示すのはsource repositoryの変更だけです。`chezmoi status`は
render後のtargetとlive homeの差分を示します。そのため、Gitがcleanでも
現在のPCが同期済みとは限りません。

Chezemonは4層を一つのリスク順レビューキューへ統合します。

- chezmoiの2文字statusを普通の言葉で説明
- 両側で変わったファイルを通常の適用候補より優先
- 現在フェーズと「次にやるべき一手」を表示
- 残りのフローを「対応不要・現在・待機中」で表示（未着手を完了とは表示しない）
- 1ファイルずつ色分けdiffを確認（256 KiBを超える分は切り詰め）
- スクリプトを通常のファイル変更と分離し、source側の名前と実行タイミングを表示
- 秘密情報を含むdiffを明示操作までマスク
- source Git、upstream位置、最近のcommitを表示

## 推奨フロー

Chezemonは最新snapshotから、次の6段階を組み立てます。

1. 両側で変更されたファイルを保護する
2. live側だけの変更を残すか決める
3. source Gitのworking treeを安定させる
4. source履歴とupstreamを同期する
5. スクリプトと副作用を確認する
6. 望ましいファイル変更を確認して適用する

stageを選ぶと対応するキューへ移動します。ファイルを選ぶと、状態の意味、
最も安全な次の操作、diffが表示されます。stageを解決してRefreshすると、
Chezemonが次のフェーズへ自動的に進みます。

読み取り専用UIと組み合わせて使う手動commandや安全ルールは、
[推奨chezmoi運用](docs/recommended-workflow.md)を参照してください。

## CLI snapshot

ローカルserverを起動せず、同じ状態モデルをJSONで確認できます。

```bash
go run ./cmd/chezemon --snapshot
```

JSONにはmetadataとpathが含まれますが、ファイル内容やdiff内容は含まれません。
将来のVS Code拡張、desktop、TUIもこの共通interfaceを利用する予定です。

## 安全設計

- 読み取り専用: dotfilesとsource repositoryへ書き込む経路は存在しない
- HTTP listenerはloopback限定。loopback以外の`--listen`は終了コード2で拒否
- loopback以外の`Host`ヘッダを403で拒否。悪意あるページが自ドメインを
  127.0.0.1へ向けてAPIへ到達すること（DNS rebinding）を防ぐ
- ルートは`GET`のみ。それ以外のメソッドは405
- shell文字列を組み立てない
- chezmoi内蔵diffを使用
- 秘密diffをデフォルトでマスク。判定はchezmoi自身の暗号化一覧とファイル名の
  ヒューリスティックの2系統
- diffはレビューキューにあるパスに限定
- telemetryや外部へのファイル送信なし
- 厳格なContent Security Policy

将来の書き込み操作では、毎回最新planを作り、previewと明示確認を行い、
対象を限定し、backup・秘密情報scan・操作journalを残します。

## 開発

Goが入っている場合：

```bash
go test ./...
go run ./cmd/chezemon --open
```

Nixとdirenvを使う場合：

```bash
direnv allow
go test ./...
go build ./cmd/chezemon
```

第三者製のGo packageやbrowser runtimeへの依存はありません。

CIではWindows、macOS、Linuxでrace detector付きtestとbuildを実行し、
golangci-lintを別jobで実行します。リリース用のamd64/arm64バイナリは
release workflowがcross buildし、タグと一致する刻印が無ければ公開を中止します。

## 関連ドキュメント

- [不満・既存ツールの調査](docs/research.md)
- [製品設計と安全設計](docs/product.md)
- [推奨chezmoi運用](docs/recommended-workflow.md)

Chezemonは独立したprojectであり、chezmoi projectとは無関係です。
