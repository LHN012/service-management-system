import { Plus } from 'lucide-react'
import { ProjectTable } from '../components/ProjectTable.jsx'

export function Projects({ projects, loading, onNew, onOpen, onEdit, onOperate }) {
  return (
    <section className="work-section page-section">
      <div className="section-heading section-heading--page">
        <div><h2>项目</h2><p>管理项目配置、后端进程、前端 Nginx 和部署规则</p></div>
        <button className="button button--primary" onClick={onNew}><Plus size={16} />新建项目</button>
      </div>
      <ProjectTable projects={projects} loading={loading} onOpen={onOpen} onEdit={onEdit} onOperate={onOperate} />
    </section>
  )
}
