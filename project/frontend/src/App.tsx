import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import RepositoryInputPage from './pages/RepositoryInputPage/RepositoryInputPage';
import AnalysisStatusPage from './pages/AnalysisPages/AnalysisStatusPage';
import ReportPage from './pages/ReportPage/ReportPage';
import ErrorPage from './pages/ErrorPages/ErrorPage';

const router = createBrowserRouter([
  {
    path: '/',
    element: <RepositoryInputPage />,
    errorElement: <ErrorPage />,
  },
  {
    path: '/analysis/:id',
    element: <AnalysisStatusPage />,
    errorElement: <ErrorPage />,
  },
  {
    path: '/report/:id',
    element: <ReportPage />,
    errorElement: <ErrorPage />,
  },
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
  return <RouterProvider router={router} />;
}

export default App;
