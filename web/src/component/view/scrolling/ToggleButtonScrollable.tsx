import ArrowDropDownIcon from "@mui/icons-material/ArrowDropDown"
import {Box, Button, Checkbox, ListItemText, Menu, MenuItem, SxProps, Tooltip} from "@mui/material"
import {Theme} from "@mui/material/styles"
import {cloneElement, MouseEvent, ReactElement, useEffect, useState} from "react"

import {SxPropsMap} from "../../../app/type"

const ALL = "ALL"
const PRIMARY_TAGS = new Set([
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
const SX: SxPropsMap = {
    root: {display: "flex", alignItems: "center", whiteSpace: "nowrap"},
    selector: {display: "flex", alignItems: "center", gap: "6px", marginRight: "8px"},
    menuPaper: {maxHeight: "360px"},
    menuItem: {padding: "2px 10px", minHeight: "30px"},
    checkbox: {padding: "2px 6px 2px 0"},
    label: {overflow: "hidden", textOverflow: "ellipsis", maxWidth: "260px"},
    after: {display: "flex", gap: "3px", marginLeft: "auto"},
    element: {padding: "3px 7px", borderRadius: "3px", lineHeight: "1.1"},
    button: {
        ...{padding: "3px 7px", borderRadius: "3px", lineHeight: "1.1"},
        minWidth: "86px",
        justifyContent: "space-between",
        textTransform: "none",
    },
}

type Props = {
    tags: string[],
    selected: string[],
    renderActions?: ReactElement<{sx?: SxProps<Theme>}>[],
    onUpdate: (tags: string[]) => void,
}

export function ToggleButtonScrollable(props: Props) {
    const {tags, selected, onUpdate, renderActions} = props
    const canonicalTagMap = new Map(tags.map(tag => [tag.toLowerCase(), tag]))
    const canonicalSelected = selected.map(canonicalTag)
    const tagsSet = new Set(tags)
    const [selectedSet, setSelectedSet] = useState(new Set(canonicalSelected))
    const [primaryAnchorEl, setPrimaryAnchorEl] = useState<HTMLElement | null>(null)
    const [secondaryAnchorEl, setSecondaryAnchorEl] = useState<HTMLElement | null>(null)

    const isAll = selectedSet.has(ALL)
    const count = isAll ? "0" : selectedSet.size.toString()
    const primaryTags = tags.filter(isPrimaryTag)
    const secondaryTags = tags.filter(tag => !isPrimaryTag(tag))
    const removedPrimaryTags = canonicalSelected.filter(tag => !tagsSet.has(tag) && isPrimaryTag(tag))
    const removedSecondaryTags = canonicalSelected.filter(tag => !tagsSet.has(tag) && !isPrimaryTag(tag))

    useEffect(() => {
        setSelectedSet(new Set(canonicalSelected))
        if (!sameTags(selected, canonicalSelected)) onUpdate(canonicalSelected)
    }, [selected, tags])

    return (
        <Box sx={SX.root}>
            <Box sx={SX.selector}>
                {renderDropdown("Environment", primaryTags, removedPrimaryTags, primaryAnchorEl, setPrimaryAnchorEl, true)}
                {renderDropdown("Tenant", secondaryTags, removedSecondaryTags, secondaryAnchorEl, setSecondaryAnchorEl)}
            </Box>
            {renderAfter()}
        </Box>
    )

    function renderDropdown(
        label: string,
        list: string[],
        removedList: string[],
        anchorEl: HTMLElement | null,
        setAnchorEl: (value: HTMLElement | null) => void,
        withAll: boolean = false,
    ) {
        if (list.length === 0 && removedList.length === 0 && !withAll) return null
        const selectedList = [...selectedSet].filter(tag => list.includes(tag) || removedList.includes(tag))
        const open = Boolean(anchorEl)

        return (
            <>
                <Tooltip title={renderTagsTooltip()} placement={"top"}>
                    <Button
                        sx={SX.button}
                        color={"secondary"}
                        size={"small"}
                        variant={selectedList.length > 0 ? "contained" : "outlined"}
                        endIcon={<ArrowDropDownIcon fontSize={"small"}/>}
                        onClick={e => setAnchorEl(e.currentTarget)}
                    >
                        {renderButtonLabel(label, selectedList, withAll)}
                    </Button>
                </Tooltip>
                <Menu
                    anchorEl={anchorEl}
                    open={open}
                    onClose={() => setAnchorEl(null)}
                    slotProps={{paper: {sx: SX.menuPaper}}}
                >
                    {withAll ? renderMenuItem(ALL, isAll, handleClickAll) : null}
                    {list.map(tag => renderMenuItem(tag, selectedSet.has(tag), handleClick))}
                    {removedList.map(tag => renderRemovedMenuItem(tag, handleClick))}
                </Menu>
            </>
        )
    }

    function renderAfter() {
        return (
            <Box sx={SX.after}>
                {renderInfo()}
                <Tooltip title={renderTagsTooltip()} placement={"top"}>
                    <span>{renderCountButton()}</span>
                </Tooltip>
            </Box>
        )
    }

    function renderTagsTooltip() {
        return (
            <Box>
                <Box><b>Tags Selected</b></Box>
                <Box>[ use <b>ctrl</b> to pick more than one tag ]</Box>
            </Box>
        )
    }

    function renderInfo() {
        if (renderActions === undefined) return
        return renderActions.map(e => cloneElement(e, {sx: SX.element}))
    }

    function renderButtonLabel(label: string, list: string[], withAll: boolean) {
        if (withAll && isAll) return `${label}: ${ALL}`
        if (list.length === 0) return label
        const value = list.join(", ")
        return <Box component={"span"} sx={SX.label}>{label}: {value}</Box>
    }

    function renderCountButton() {
        return (
            <Button
                sx={SX.element}
                color={"secondary"}
                size={"small"}
                disabled
            >
                {count}
            </Button>
        )
    }

    function renderMenuItem(tag: string, checked: boolean, onClick: (e: MouseEvent<HTMLElement>, value: string) => void) {
        return (
            <MenuItem
                sx={SX.menuItem}
                key={tag}
                selected={checked}
                value={tag}
                onClick={(e) => onClick(e, tag)}
            >
                <Checkbox sx={SX.checkbox} size={"small"} color={"secondary"} checked={checked}/>
                <ListItemText primary={tag}/>
            </MenuItem>
        )
    }

    function renderRemovedMenuItem(tag: string, onClick: (e: MouseEvent<HTMLElement>, value: string) => void) {
        if (tag === ALL) return null
        return (
            <MenuItem
                sx={SX.menuItem}
                key={tag}
                selected
                value={tag}
                onClick={(e) => onClick(e, tag)}
            >
                <Checkbox sx={SX.checkbox} size={"small"} color={"error"} checked/>
                <ListItemText primary={tag}/>
            </MenuItem>
        )
    }

    function handleClick(_: MouseEvent, value: string) {
        const tmp = new Set(selectedSet)
        if (tmp.has(value)) {
            tmp.delete(value)
            if (tmp.size === 0) tmp.add(ALL)
        } else {
            tmp.delete(ALL)
            tmp.add(value)
        }
        setSelectedSet(tmp)
        onUpdate([...tmp])
    }

    function handleClickAll() {
        const tmp = new Set([ALL])
        setSelectedSet(tmp)
        onUpdate([...tmp])
        setPrimaryAnchorEl(null)
        setSecondaryAnchorEl(null)
    }

    function isPrimaryTag(tag: string) {
        return PRIMARY_TAGS.has(tag.toLowerCase())
    }

    function canonicalTag(tag: string) {
        if (tag === ALL) return tag
        return canonicalTagMap.get(tag.toLowerCase()) ?? tag
    }

    function sameTags(left: string[], right: string[]) {
        if (left.length !== right.length) return false
        return left.every((tag, index) => tag === right[index])
    }
}
