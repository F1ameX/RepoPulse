import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import RepositoryInputPage from './pages/RepositoryInputPage';
import AnalysisStatusPage from './pages/AnalysisStatusPage';
import ReportPage from './pages/ReportPage';
import ErrorPage from './pages/ErrorPage';

const router = createBrowserRouter([
  {
    path: '/',
    element: <RepositoryInputPage />,
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
]);

function App() {
  return <RouterProvider router={router} />;
}

export default App;
