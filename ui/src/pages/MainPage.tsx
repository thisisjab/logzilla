import Header from '../components/Shared/Header'
import Footer from '../components/Shared/Footer'
import Divider from '../components/Shared/Divider'
import QuerySection from '../components/Logs/QuerySection'
import QueryResult from '../components/Logs/QueryResult'

function MainPage() {
    return (
        <>
            <Header />
            <Divider />
            <main className="max-w-full text-nowrap m-5 grow flex flex-col overflow-hidden gap-3">
                <QuerySection />
                <QueryResult />
            </main >
            <Footer />
        </>
    )
}

export default MainPage
