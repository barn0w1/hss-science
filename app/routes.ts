import { type RouteConfig, index, route, prefix } from "@react-router/dev/routes";

export default [
  // 公開ページ (SSR)
  index("routes/_index.tsx"),
  
  // 管理画面 (SPA)
  ...prefix("admin", [
    index("routes/admin/_index.tsx"),
  ]),
] satisfies RouteConfig;
