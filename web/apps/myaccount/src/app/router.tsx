import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AccountLayout } from './layouts/AccountLayout';
import { DashboardPage } from '@/pages/DashboardPage';
import { ProfilePage } from '@/pages/ProfilePage';
import { SecurityPage } from '@/pages/SecurityPage';
import { AccountSettingsPage } from '@/pages/AccountSettingsPage';

export const AppRouter = () => {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AccountLayout />}>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/profile" element={<ProfilePage />} />
          <Route path="/security" element={<SecurityPage />} />
          <Route path="/account" element={<AccountSettingsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
};
