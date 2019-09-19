# Name

# Overview
GCPのCloud Function上で動くSlack Botです。Cloud Storage上の辞書ファイルを読み込んで形態素解析を行い、Datastore上のデータに基づいて応答します。
形態素解析にはgo製のライブラリの[kagome](https://github.com/ikawaha/kagome)を使わせてもらっています。
kagomeはlocalファイルからの読み込みしか対応していなかったため、Cloud Storageから読み込めるように拡張しています。

# how to deploy
```
gcloud functions deploy slack-bot --entry-point SlackBot --runtime go111 --region asia-northeast1 --trigger-http
```

# how to add dictionary
Cloud Storageにslack-botの名前でBucketを作成し、そこにdictionary.txtという名前のファイルをおくと読み込みます。
