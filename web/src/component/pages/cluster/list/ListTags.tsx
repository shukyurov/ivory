import {Box, InputBase, ToggleButton} from "@mui/material"
import {useMemo} from "react"

import {Cluster} from "../../../../api/cluster/type"
import {SxPropsMap} from "../../../../app/type"
import {useStore, useStoreAction} from "../../../../provider/StoreProvider"
import {ToggleButtonScrollable} from "../../../view/scrolling/ToggleButtonScrollable"

const SX: SxPropsMap = {
    tags: {position: "relative", height: 0, top: "-37px"},
    input: {padding: "0px", width: "100px", height: "14px", fontSize: "14px"},
}

type Props = {
    list: Cluster[],
}

export function ListTags(props: Props) {
    const tags = useMemo(() => extractNameTags(props.list), [props.list])
    const poolTags = useMemo(() => extractPoolTags(props.list), [props.list])
    const warnings = useStore(s => s.warnings)
    const search = useStore(s => s.searchCluster)
    const activeTags = useStore(s => s.activeTags)
    const {setTags, setSearchCluster} = useStoreAction

    const warningsCount = Object.values(warnings).filter(it => it).length

    return (
        <Box sx={SX.tags}>
            <ToggleButtonScrollable
                tags={tags}
                poolTags={poolTags}
                selected={activeTags}
                onUpdate={setTags}
                renderActions={renderActions()}
            />
        </Box>
    )

    function renderActions() {
        return [
            <InputBase
                key={"search"}
                type={"text"}
                size={"small"}
                slotProps={{input: {sx: SX.input}}}
                placeholder={"Filter by name"}
                value={search}
                onChange={e => setSearchCluster(e.target.value)}
            />,
            <ToggleButton
                key={"warnings"}
                sx={SX.element}
                color={"warning"}
                size={"small"}
                selected={warningsCount > 0}
                disabled
                value={warnings}
            >
                {warningsCount}
            </ToggleButton>
        ]
    }
}

export function getClusterPool(cluster: Cluster) {
    return cluster.sidecars
        .map(sidecar => sidecar.host.trim())
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b))
        .join(" / ")
}

function extractNameTags(list: Cluster[]) {
    const tagMap = new Map<string, string>()
    for (const cluster of list) {
        const [environment, ...tenantParts] = cluster.name.split("-")
        addTag(environment)
        addTag(tenantParts.join("-"))
    }
    return [...tagMap.values()].sort((a, b) => a.localeCompare(b))

    function addTag(tag: string) {
        const value = tag.trim()
        if (!value) return
        const key = value.toLowerCase()
        if (!tagMap.has(key)) tagMap.set(key, value)
    }
}

function extractPoolTags(list: Cluster[]) {
    const poolMap = new Map<string, string>()
    for (const cluster of list) {
        const pool = getClusterPool(cluster)
        if (!pool) continue
        const key = pool.toLowerCase()
        if (!poolMap.has(key)) poolMap.set(key, pool)
    }
    return [...poolMap.values()].sort((a, b) => a.localeCompare(b))
}
