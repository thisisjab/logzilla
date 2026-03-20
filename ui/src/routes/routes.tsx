import { Route, Routes } from 'react-router-dom'
import MainPage from '../pages/MainPage'
import HealthcheckPage from '../pages/HealthcheckPage'

function AllRoutes() {
    return (
        <Routes>
            <Route path="/" element={<MainPage />} />
            <Route path="/healthcheck" element={<HealthcheckPage />} />
        </Routes>
    )
}

export default AllRoutes
