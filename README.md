# cloud-gaming-operator

GCE上に建てたクラウドゲーミング用のVMを管理するやつ

## 事前準備

- GCEのインスタンスかマシンイメージを作成しておく
- `GOOGLE_APPLICATION_CREDENTIALS`: サービスアカウントのパスを指定

## コマンド

```text
NAME:
   cloud-gaming-operator - GCEに立てているクラウドゲーミング用のインスタンスを管理するCLIツール

USAGE:
   cloud-gaming-operator [global options] command [command options] [arguments...]

COMMANDS:
   list, l    現在起動中のインスタンスを表示する。
   create, c  マシンイメージからインスタンスを起動する。すでに起動している場合は何もしない。
   remove, r  マシーンイメージを作成して、起動中のインスタンスを削除する。
   help, h    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --projectID value, -p value  GCPのプロジェクトIDを指定する (default: "cloud-gaming-p1ass") [$CLOUD_GAMING_OPERATOR_PROJECT_ID]
   --region value               GCPのリージョンを指定する (デフォルト: asia-northeast1) [$CLOUD_GAMING_OPERATOR_REGION]
   --zone value                 GCPのプロジェクトIDを指定する (デフォルト: asia-northeast1-a) [$CLOUD_GAMING_OPERATOR_ZONE]
   --help, -h                   show help (default: false)

```