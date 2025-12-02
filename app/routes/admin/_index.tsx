import type { Route } from "./+types/_index";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "管理画面 | HSS Science" },
  ];
}

export default function AdminDashboard() {
  return (
    <div className="min-h-screen bg-gray-100 dark:bg-gray-800">
      <header className="bg-white dark:bg-gray-900 shadow">
        <div className="container mx-auto px-4 py-4">
          <h1 className="text-xl font-bold text-gray-900 dark:text-white">
            HSS Science 管理画面
          </h1>
        </div>
      </header>
      
      <main className="container mx-auto px-4 py-8">
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          <DashboardCard 
            title="記事管理" 
            description="ニュースや記事の作成・編集" 
            href="/admin/articles" 
          />
          <DashboardCard 
            title="メンバー管理" 
            description="部員情報の管理" 
            href="/admin/members" 
          />
          <DashboardCard 
            title="メディア" 
            description="画像ファイルの管理" 
            href="/admin/media" 
          />
        </div>
      </main>
    </div>
  );
}

function DashboardCard({ title, description, href }: { 
  title: string; 
  description: string; 
  href: string; 
}) {
  return (
    <a 
      href={href}
      className="block p-6 bg-white dark:bg-gray-900 rounded-lg shadow hover:shadow-md transition-shadow"
    >
      <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
        {title}
      </h2>
      <p className="mt-2 text-gray-600 dark:text-gray-400">
        {description}
      </p>
    </a>
  );
}
