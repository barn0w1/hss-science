import { defineConfig } from 'orval';

export default defineConfig({
  // Accounts Service
  accounts: {
    input: {
      target: './openapi/public/accounts/v1/accounts.swagger.json',
    },
    output: {
      mode: 'split',
      target: './src/gen/accounts/accounts.ts',
      schemas: './src/gen/accounts/model',
      client: 'react-query',
      httpClient: 'axios',
      override: {
        mutator: {
          path: './src/clients/service-clients.ts',
          name: 'accountsClient',
        },
      },
    },
  },
});