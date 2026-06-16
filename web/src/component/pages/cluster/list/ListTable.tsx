import {Box, Table, TableCell, TableHead, TableRow} from "@mui/material"
import {useMemo, useState} from "react"

import {ClusterApi} from "../../../../api/cluster/router"
import {Cluster} from "../../../../api/cluster/type"
import {Permission} from "../../../../api/permission/type"
import {SxPropsMap} from "../../../../app/type"
import {SxPropsFormatter} from "../../../../app/utils"
import {useStore} from "../../../../provider/StoreProvider"
import scroll from "../../../../style/scroll.module.css"
import {AlertCentered} from "../../../view/box/AlertCentered"
import {AddIconButton} from "../../../view/button/IconButtons"
import {TableBody} from "../../../view/table/TableBody"
import {TableCellLoader} from "../../../view/table/TableCellLoader"
import {Access} from "../../../widgets/access/Access"
import {Refresher} from "../../../widgets/refresher/Refresher"
import {ListCreateAuto} from "./ListCreateAuto"
import {ListRow} from "./ListRow"
import {ListRowNew} from "./ListRowNew"
import {getClusterPool} from "./ListTags"

const SX: SxPropsMap = {
    box: {overflowY: "scroll"},
    table: {"tr:last-child td": {border: 0}, "tr td, th": {padding: "5px 10px"}},
    refresh: {padding: "0px 5px"},
}

const ENVIRONMENT_TAGS = new Set([
    "dev",
    "develop",
    "development",
    "prod",
    "production",
    "stage",
    "staging",
    "test",
    "testing",
    "qa",
    "uat",
    "preprod",
    "pre-prod",
])

type Props = {
    list: Cluster[],
    pending: boolean,
    fetching: boolean,
}

export function ListTable(props: Props) {
    const activeCluster = useStore(s => s.activeCluster)
    const search = useStore(s => s.searchCluster)
    const activeTags = useStore(s => s.activeTags)
    const {list, fetching, pending} = props
    const [showNewElement, setShowNewElement] = useState(false)
    const [editNode, setEditNode] = useState("")

    const rows = useMemo(
        () => list.filter((c) => c.name.includes(search) && matchesTagFilter(c, activeTags)),
        [list, search, activeTags],
    )

    return (
        <Box sx={SX.box} className={scroll.tiny} maxHeight={activeCluster ? "25vh" : "60vh"}>
            <Table size={"small"} sx={SX.table} stickyHeader>
                <TableHead>
                    <TableRow>
                        <TableCell sx={SxPropsFormatter.style.paper} width={"220px"}>Cluster Name</TableCell>
                        <TableCellLoader
                            sx={SxPropsFormatter.style.paper}
                            label={"Instances"}
                            colSpan={2}
                            loading={fetching && !pending}
                        >
                            <Box sx={SX.refresh}>
                                <Refresher queryKeys={[ClusterApi.list.key(), ClusterApi.overview.key()]}/>
                            </Box>
                            <ListCreateAuto/>
                            <Access permission={Permission.ManageClusterUpdate}>
                                <AddIconButton
                                    tooltip={"Add Cluster Manually"}
                                    onClick={() => setShowNewElement(true)}
                                    disabled={showNewElement}
                                />
                            </Access>
                        </TableCellLoader>
                    </TableRow>
                </TableHead>
                <TableBody isLoading={pending} cellCount={3} height={32}>
                    <ListRowNew show={showNewElement} close={() => setShowNewElement(false)}/>
                    {renderRemovedRow()}
                    {renderRows()}
                    {renderEmpty()}
                </TableBody>
            </Table>
        </Box>
    )

    function renderRemovedRow() {
        if (!activeCluster) return
        if (rows.some(e => e.name === activeCluster.cluster.name)) return
        return (
            <ListRow cluster={activeCluster.cluster} editable={false}/>
        )
    }

    function renderRows() {
        return rows.map((cluster) => {
            const editable = cluster.name === editNode
            const toggle = () => setEditNode(editable ? "" : cluster.name)
            return (
                <ListRow key={cluster.name} cluster={cluster} editable={editable} toggle={toggle}/>
            )
        })
    }

    function renderEmpty() {
        if (pending || showNewElement || rows.length || activeCluster) return
        const text = search ? (
            "There are no clusters that match your filter"
        ) : (
            "There are no clusters yet. You can add them manually or by auto detection"
        )
        return (
            <TableRow>
                <TableCell colSpan={3}>
                    <AlertCentered text={text}/>
                </TableCell>
            </TableRow>
        )
    }

    function matchesTagFilter(cluster: Cluster, tags: string[]) {
        if (tags.includes("ALL")) return true

        const normalizedTags = tags.map(tag => tag.toLowerCase())
        const environmentFilters = normalizedTags.filter(tag => ENVIRONMENT_TAGS.has(tag))
        const poolFilters = normalizedTags.filter(isPoolTag)
        const tenantFilters = normalizedTags.filter(tag => !ENVIRONMENT_TAGS.has(tag) && !isPoolTag(tag))
        const nameParts = cluster.name.split("-")
        const clusterEnvironment = nameParts[0]?.toLowerCase()
        const clusterTenant = nameParts.slice(1).join("-").toLowerCase()
        const clusterPool = getClusterPool(cluster).toLowerCase()
        const clusterTags = new Set((cluster.tags ?? []).map(tag => tag.toLowerCase()))

        if (clusterEnvironment) clusterTags.add(clusterEnvironment)
        if (clusterTenant) clusterTags.add(clusterTenant)
        if (clusterPool) clusterTags.add(clusterPool)

        const matchesEnvironment = environmentFilters.length === 0 || environmentFilters.some(tag => clusterTags.has(tag))
        const matchesTenant = tenantFilters.length === 0 || tenantFilters.some(tag => clusterTags.has(tag))
        const matchesPool = poolFilters.length === 0 || poolFilters.some(tag => clusterTags.has(tag))
        return matchesEnvironment && matchesTenant && matchesPool
    }

    function isPoolTag(tag: string) {
        return tag.includes(" / ")
    }
}
