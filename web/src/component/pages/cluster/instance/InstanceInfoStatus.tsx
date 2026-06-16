import {Box, Skeleton} from "@mui/material"

import {Role} from "../../../../api/instance/type"
import {SxPropsMap} from "../../../../app/type"
import {getRoleColor, getRoleDisplay} from "../../../../app/utils"

const SX: SxPropsMap = {
    instanceStatusBlock: {
        display: "flex", alignItems: "center", justifyContent: "center",
        height: "120px", minWidth: "250px", borderRadius: "4px",
        color: "white", fontSize: "24px", fontWeight: 900,
    },
}

type Props = {
    role?: Role,
    displayRole?: string,
    loading?: boolean,
}

export function InstanceInfoStatus(props: Props) {
    const {role, displayRole, loading} = props
    if (loading) return <Skeleton variant={"rectangular"} sx={SX.instanceStatusBlock}/>
    const roleColor = getRoleColor({role, displayRole})
    const textColor = roleColor.label === "warning" ? "black" : "white"

    return (
        <Box sx={{...SX.instanceStatusBlock, background: roleColor.color, color: textColor}}>
            {getRoleDisplay({role, displayRole})}
        </Box>
    )
}
