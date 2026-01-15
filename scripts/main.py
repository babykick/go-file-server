import os
import shutil

from pypinyin import lazy_pinyin
import typer

app = typer.Typer(help="吉他谱文件整理工具")


def get_first_letter(name: str) -> str:
    """获取名字的首字母并大写"""
    pinyin_list = lazy_pinyin(name)
    if pinyin_list:
        return pinyin_list[0][0].upper()
    return ""


@app.command()
def organize(
    source_dir: str = typer.Option(
        "/mnt/MOBILEDISK1/吉他/卓著谱/卓著谱/W无歌手名/",
        "--source", "-s",
        help="源目录路径"
    ),
    target_dir: str = typer.Option(
        "/mnt/MOBILEDISK1/吉他/卓著谱/卓著谱/",
        "--target", "-t",
        help="目标基础目录路径"
    ),
    dry_run: bool = typer.Option(
        False,
        "--dry-run", "-n",
        help="仅显示将要执行的操作，不实际移动文件"
    ),
):
    """整理文件：按歌手名首字母分类移动文件"""
    if not os.path.exists(source_dir):
        typer.echo(f"源目录不存在: {source_dir}")
        raise typer.Exit(1)

    files = os.listdir(source_dir)
    typer.echo(f"共找到 {len(files)} 个文件")

    for filename in files:
        file_path = os.path.join(source_dir, filename)

        if not os.path.isfile(file_path):
            continue

        if "_" not in filename:
            continue

        parts = filename.split("_", 1)
        if len(parts) < 2:
            typer.echo(f"跳过（分割失败）: {filename}")
            continue

        song_name = parts[0]
        singer_with_ext = parts[1]
        singer = os.path.splitext(singer_with_ext)[0]

        if not singer:
            typer.echo(f"跳过（歌手名为空）: {filename}")
            continue

        first_letter = get_first_letter(singer)
        if not first_letter:
            typer.echo(f"跳过（无法获取首字母）: {filename}")
            continue

        if 'A' <= singer[0].upper() <= 'Z':
            target_folder_name = singer
        else:
            target_folder_name = f"{first_letter}{singer}"

        target_folder_path = os.path.join(target_dir, target_folder_name)

        if dry_run:
            typer.echo(f"[DRY-RUN] 将移动: {filename} -> {target_folder_name}/")
            continue

        if not os.path.exists(target_folder_path):
            try:
                os.makedirs(target_folder_path)
                typer.echo(f"已创建目录: {target_folder_name}")
            except Exception as e:
                typer.echo(f"创建目录失败: {target_folder_name}, 错误: {e}")
                continue

        target_file_path = os.path.join(target_folder_path, filename)
        try:
            shutil.move(file_path, target_file_path)
            typer.echo(f"已移动: {filename} -> {target_folder_name}/")
        except Exception as e:
            typer.echo(f"移动失败: {filename}, 错误: {e}")


@app.command()
def list_files(
    source_dir: str = typer.Option(
        "/mnt/MOBILEDISK1/吉他/卓著谱/卓著谱/W无歌手名/",
        "--source", "-s",
        help="源目录路径"
    ),
):
    """列出源目录中的所有文件"""
    if not os.path.exists(source_dir):
        typer.echo(f"源目录不存在: {source_dir}")
        raise typer.Exit(1)

    files = os.listdir(source_dir)
    typer.echo(f"共找到 {len(files)} 个文件:")
    for filename in files:
        file_path = os.path.join(source_dir, filename)
        if os.path.isfile(file_path):
            typer.echo(f"  {filename}")


if __name__ == "__main__":
    app()
