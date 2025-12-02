import type { Route } from "./+types/_index";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "HSS Science" },
    { name: "description", content: "科学部ホームページ" },
  ];
}

export function loader({ context }: Route.LoaderArgs) {
  return { message: "Welcome to HSS Science" };
}

export default function Home({ loaderData }: Route.ComponentProps) {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <main className="container mx-auto px-4 py-16">
        <h1 className="text-4xl font-bold text-gray-900 dark:text-white">
          {loaderData.message}
        </h1>
        <p className="mt-4 text-gray-600 dark:text-gray-300">
          科学部の公式ホームページです
        </p>
      </main>
    </div>
  );
}
