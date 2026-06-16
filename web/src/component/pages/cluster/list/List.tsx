import {useRouterClusterList} from "../../../../api/cluster/hook"
import {Permission} from "../../../../api/permission/type"
import {ErrorSmart} from "../../../view/box/ErrorSmart"
import {PageMainBox} from "../../../view/box/PageMainBox"
import {Access} from "../../../widgets/access/Access"
import {ListTable} from "./ListTable"
import {ListTags} from "./ListTags"

export function List() {
    const clusters = useRouterClusterList(["ALL"])

    return (
        <PageMainBox withMarginTop={"40px"}>
            <Access permission={Permission.ViewTagList}><ListTags list={clusters.data ?? []}/></Access>
            {clusters.error ? <ErrorSmart error={clusters.error}/> : (
                <ListTable list={clusters.data ?? []} fetching={clusters.isFetching} pending={clusters.isPending}/>
            )}
        </PageMainBox>
    )
}
