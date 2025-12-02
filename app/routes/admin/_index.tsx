import type { Route } from "./+types/_index";
import { 
  AdminLayout, 
  AdminPageHeader, 
  AdminCard,
  AdminButton 
} from "~/components/admin/AdminLayout";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "管理画面 | HSS Science" },
  ];
}

// Stats data (placeholder)
const stats = [
  { label: "公開記事", value: "12", change: "+2", changeType: "positive" },
  { label: "下書き", value: "3", change: null, changeType: null },
  { label: "メンバー", value: "24", change: "+5", changeType: "positive" },
  { label: "メディア", value: "156", change: "+23", changeType: "positive" },
];

// Recent articles (placeholder)
const recentArticles = [
  { id: 1, title: "2024年度 文化祭展示について", status: "published", date: "2024-11-28" },
  { id: 2, title: "物理班 ロボットコンテスト結果報告", status: "published", date: "2024-11-25" },
  { id: 3, title: "化学班 実験レポート", status: "draft", date: "2024-11-20" },
  { id: 4, title: "生物班 フィールドワーク報告", status: "published", date: "2024-11-15" },
];

export default function AdminDashboard() {
  return (
    <AdminLayout>
      <AdminPageHeader 
        title="ダッシュボード"
        description="HSS Science 管理画面"
      />

      {/* Stats Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {stats.map((stat) => (
          <AdminCard key={stat.label} className="p-5">
            <p className="text-xs font-medium text-neutral-500 uppercase tracking-wider">
              {stat.label}
            </p>
            <p className="mt-2 text-3xl font-semibold text-neutral-900">
              {stat.value}
            </p>
            {stat.change && (
              <p className={`mt-1 text-sm ${
                stat.changeType === "positive" ? "text-green-600" : "text-red-600"
              }`}>
                {stat.change} 今月
              </p>
            )}
          </AdminCard>
        ))}
      </div>

      {/* Quick Actions */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
        <QuickActionCard
          title="記事を作成"
          description="新しいニュース記事やブログ投稿を作成"
          href="/admin/articles/new"
          icon={
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="square" strokeWidth={1.5} d="M12 4v16m8-8H4" />
            </svg>
          }
        />
        <QuickActionCard
          title="メディアをアップロード"
          description="画像やファイルをアップロード"
          href="/admin/media/upload"
          icon={
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="square" strokeWidth={1.5} d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M12 4v12m0 0l-4-4m4 4l4-4" />
            </svg>
          }
        />
        <QuickActionCard
          title="メンバーを追加"
          description="新しい部員情報を登録"
          href="/admin/members/new"
          icon={
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="square" strokeWidth={1.5} d="M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z" />
            </svg>
          }
        />
      </div>

      {/* Recent Articles */}
      <AdminCard>
        <div className="px-5 py-4 border-b border-neutral-200 flex items-center justify-between">
          <h2 className="font-semibold text-neutral-900">最近の記事</h2>
          <AdminButton variant="ghost" size="sm">
            すべて見る →
          </AdminButton>
        </div>
        <div className="divide-y divide-neutral-200">
          {recentArticles.map((article) => (
            <div key={article.id} className="px-5 py-4 flex items-center justify-between hover:bg-neutral-50 transition-colors">
              <div className="flex items-center gap-4">
                <div className={`w-2 h-2 ${
                  article.status === "published" ? "bg-green-500" : "bg-amber-500"
                }`} />
                <div>
                  <p className="font-medium text-neutral-900">{article.title}</p>
                  <p className="text-sm text-neutral-500">{article.date}</p>
                </div>
              </div>
              <span className={`text-xs font-medium px-2 py-1 ${
                article.status === "published" 
                  ? "bg-green-100 text-green-700"
                  : "bg-amber-100 text-amber-700"
              }`}>
                {article.status === "published" ? "公開" : "下書き"}
              </span>
            </div>
          ))}
        </div>
      </AdminCard>
    </AdminLayout>
  );
}

function QuickActionCard({ 
  title, 
  description, 
  href, 
  icon 
}: { 
  title: string; 
  description: string; 
  href: string;
  icon: React.ReactNode;
}) {
  return (
    <a href={href}>
      <AdminCard className="p-5 hover:border-neutral-400 transition-colors cursor-pointer group">
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 bg-neutral-900 text-white flex items-center justify-center group-hover:bg-neutral-700 transition-colors">
            {icon}
          </div>
          <div>
            <h3 className="font-semibold text-neutral-900">{title}</h3>
            <p className="text-sm text-neutral-500 mt-1">{description}</p>
          </div>
        </div>
      </AdminCard>
    </a>
  );
}
