import { type RouteConfig, route, index } from "@react-router/dev/routes";

export default [
  index("./routes/_index.tsx"),
  route("login", "./routes/login.tsx"),
  route("registration", "./routes/registration.tsx"),
  route("logout", "./routes/logout.tsx"),
  route("settings", "./routes/settings.tsx"),
  route("recovery", "./routes/recovery.tsx"),
  route("verification", "./routes/verification.tsx"),
  route("error", "./routes/error.tsx"),
] satisfies RouteConfig;
