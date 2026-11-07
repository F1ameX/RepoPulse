import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { AuthProvider } from '@/components/Auth/AuthProvider';
import RepositoryInputPage from './pages/RepositoryInputPage/RepositoryInputPage';
import AnalysisStatusPage from './pages/AnalysisPages/AnalysisStatusPage';
import ReportPage from './pages/ReportPage/ReportPage';
import ErrorPage from './pages/ErrorPages/ErrorPage';
import AuthPage from './pages/AuthPage/AuthPage';
import HistoryPage from './pages/HistoryPage/HistoryPage';

const router = createBrowserRouter([
  {
    path: '/',
    element: <RepositoryInputPage />,
    errorElement: <ErrorPage />,
  },
  {
    path: '/analysis/:id?',
    element: <AnalysisStatusPage />,
    errorElement: <ErrorPage />,
  },
  {
    path: '/report/:id?',
    element: <ReportPage />,
    errorElement: <ErrorPage />,
  },
  { path: '/login', element: <AuthPage key="login" mode="login" /> },
  { path: '/register', element: <AuthPage key="register" mode="register" /> },
  { path: '/history', element: <HistoryPage /> },
  {
    path: '/error',
    element: <ErrorPage />,
  },
  {
    path: '*',
    element: <ErrorPage />,
  },
]);

function App() {
  return (
    <AuthProvider>
      <RouterProvider router={router} />
    </AuthProvider>
  );
}

export default App;
