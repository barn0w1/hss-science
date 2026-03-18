import { createBrowserRouter, RouterProvider, redirect } from "react-router-dom"
import { AppShell } from "@/components/layout/AppShell"
import { DrivePage } from "@/pages/DrivePage"
import { TrashPage } from "@/pages/TrashPage"
import { NotFoundPage } from "@/pages/NotFoundPage"

const router = createBrowserRouter([
  {
    element: <AppShell />,
    children: [
      { index: true, loader: () => redirect("/drive") },
      { path: "drive/:spaceId?/:nodeId?", element: <DrivePage /> },
      { path: "trash", element: <TrashPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
])

export default function App() {
  return <RouterProvider router={router} />
}
