import { defineConfig } from 'orval';

export default defineConfig({
  hss: {
    input: {
      // make proto-gen で生成された Swagger JSON を参照
      target: './openapi/api.swagger.json',
    },
    output: {
      mode: 'tags-split', // タグ(Service)ごとにファイルを分割
      target: './src/generated',
      schemas: './src/model',
      client: 'react-query', // React Query Hooksを生成
      override: {
        mutator: {
          path: './src/lib/axios.ts', // さっき作ったAxiosインスタンスを使う
          name: 'customInstance',
        },
      },
    },
  },
});