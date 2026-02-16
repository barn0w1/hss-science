// pages/ChatPage.tsx

// Layouts
import { MainLayout } from '@/app/layouts/MainLayout';
import { MainAreaLayout } from '@/app/layouts/MainAreaLayout';

export const ChatPage = () => {
  return (
    <MainLayout
      // --- ヘッダーエリア ---
      // （ここにGoogle Chromeのような検索バーが入ります）
      header={
        <div className="w-full h-full bg-white" />
      }
    >
      <MainAreaLayout
        // --- 左パネルエリア ---
        // （幅の w-80 や背景色の bg-gray-50 は CSS 側で定義済みなので、中身は空でOK）
        left={
          <div className="w-full h-full" />
        }

        // --- 右パネルエリア ---
        // （ここにチャットのタイムラインと入力欄が入ります）
        right={
          <div className="w-full h-full bg-white" />
        }
      />
    </MainLayout>
  );
};