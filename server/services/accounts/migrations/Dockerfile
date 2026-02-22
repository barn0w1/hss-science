FROM migrate/migrate
COPY . /migrations/
# ENTRYPOINT をシェル実行に上書きし、環境変数を展開させる
ENTRYPOINT ["sh", "-c", "migrate -path /migrations -database \"$DATABASE_URL\" up"]