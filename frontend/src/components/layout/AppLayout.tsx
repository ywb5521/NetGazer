import { Outlet } from 'react-router-dom';
import { AppSidebar } from './AppSidebar';

export function AppLayout() {
  return (
    <div className="flex min-h-screen">
      <AppSidebar />
      <main className="ml-56 flex-1 overflow-auto p-6">
        <Outlet />
      </main>
    </div>
  );
}
