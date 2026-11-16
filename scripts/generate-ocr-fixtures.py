"""DI-SC-16/17: self-authored OCR inputs and gold, never recognition output.

Authoring only: Pillow 13.0.0, ReportLab 4.4.9, pinned OFL NotoSansSC.
The application and CI consume frozen bytes, not these authoring dependencies.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil

import PIL
from PIL import Image, ImageDraw, ImageFilter, ImageFont
import reportlab
from reportlab.pdfgen import canvas
from reportlab.lib.utils import ImageReader

FONT_SHA = 'a3041811a78c361b1de50f953c805e0244951c21c5bd412f7232ef0d899af0da'
W, H, DPI = 1654, 2339, 200
CN = [
 ('集合与分类', '图书管理员把自然科学和文学书籍分别放在两个书架。一本书可以同时具有多个主题标签。判断两个集合是否相同，需要比较成员，不能只比较名称。', '记录共同成员', '解释分类依据'),
 ('函数与对应', '每个输入只能对应一个确定的输出。观察温度随时间变化的记录，先写清横轴与纵轴的单位，再描述变化趋势。相同输出可以来自不同输入。', '检查输入范围', '标明测量单位'),
 ('平均数与中位数', '一组数据中出现特别大的观测值时，平均数可能明显变化，中位数通常更稳定。报告统计量时，应同时说明样本数量和数据来源。', '保留异常记录', '比较两种统计量'),
 ('概率与重复试验', '一次试验的结果不能代表长期规律。把条件保持一致，重复记录事件是否发生，再比较相对频率。独立事件与互斥事件的含义不同。', '固定试验条件', '记录完整次数'),
 ('向量与方向', '速度不仅包含大小，也包含方向。在平面坐标中描述位移时，分别记录水平和竖直分量。两个分量的单位必须一致。', '画出坐标方向', '核对分量单位'),
 ('质量守恒', '封闭系统发生化学反应时，总质量保持不变。实验前后都要把容器和全部物质纳入称量范围，避免把气体逸出误认为质量消失。', '检查系统边界', '重复称量步骤'),
 ('电路与测量', '电流表应串联在待测支路，电压表应并联在被测元件两端。接线前断开电源，估计量程，并核对仪表正负接线柱。', '先断开电源', '选择合适量程'),
 ('光的反射', '入射光线、反射光线和法线位于同一平面。测量角度时，应以法线为基准。镜面位置改变以后，需要重新确定法线。', '确定法线位置', '比较两个角度'),
 ('细胞与结构', '观察植物细胞时，先使用低倍镜寻找清晰区域，再调整高倍镜。绘图只记录实际看到的结构，不把教材示意图当作实验结果。', '先用低倍观察', '区分观察和推断'),
 ('生态系统', '生产者、消费者和分解者共同参与物质循环。分析食物网时，要说明箭头代表能量流动的方向，不能把数量变化简单理解为单向因果。', '说明箭头含义', '寻找多个影响因素'),
 ('二分查找', '二分查找要求数据已经按照确定规则排序。每一步比较中间元素，并缩小仍可能包含目标的区间。空数组和未找到目标也需要明确处理。', '检查排序前提', '验证边界情况'),
 ('队列与顺序', '队列按先进入先离开的顺序处理任务。提交成功与执行完成是不同状态。系统恢复后，应根据持久记录继续处理，避免重复发布结果。', '区分任务状态', '保留持久记录'),
 ('数据库事务', '一组相关写入应当全部成功或全部撤销。事务提交以后，读取者才能看到完整结果。并发请求不能依靠先查询再插入来保证唯一性。', '检查原子提交', '使用唯一约束'),
 ('网络与超时', '网络请求可能遇到连接失败、响应缓慢或回执丢失。设置超时只是限制等待时间，并不能证明远端没有执行。重试前应核对幂等条件。', '区分失败原因', '核对幂等条件'),
 ('资料与出处', '引用资料时应保留作者、标题和具体位置。摘录内容需要与原文逐字核对。页面发生修订后，旧引用应继续对应原来的版本。', '核对原文位置', '记录资料版本'),
 ('实验与对照', '设计对照实验时，每轮只改变需要研究的条件。其余条件尽可能保持一致。观察结果与解释结论应分别记录，失败样本也不能删除。', '保持控制条件', '保留失败样本'),
 ('地图与比例尺', '比例尺表示图上距离与实际距离的关系。计算前先把单位统一。地图投影可能改变面积或角度，因此不同地图适用于不同任务。', '统一长度单位', '说明地图用途'),
 ('天气与气候', '天气描述短时间内的大气状态，气候描述较长时期的统计特征。一次降温不能单独证明长期趋势改变，比较时要使用一致的时间范围。', '区分时间尺度', '比较长期记录'),
 ('论证与证据', '观点需要能够核对的证据支持。相关关系本身不能证明因果关系。阅读材料时，应区分事实描述、推断过程和作者提出的建议。', '寻找证据来源', '区分事实和建议'),
 ('预算与计划', '制定活动预算时，分别列出固定支出和可变支出。估算数量与单价后，再检查总额是否超过上限。实际支出变化应及时记录。', '分别记录支出', '核对预算上限'),
]
EN = [
 ('Observation and inference', 'An observation records what an instrument or a person actually detects. An inference explains a possible reason for that observation. Keep both kinds of statements separate in the laboratory notebook.', 'Record the evidence', 'Explain the inference'),
 ('Units and dimensions', 'A numerical result is incomplete without its unit. Convert quantities to compatible units before adding them. Dimensional analysis can reveal a mistaken equation even when the arithmetic looks correct.', 'Check every unit', 'Compare dimensions'),
 ('Fractions and ratios', 'A fraction describes a part relative to a specified whole. A ratio compares two quantities whose order matters. Before simplifying either expression, identify what each number represents.', 'Identify the whole', 'Keep the same order'),
 ('Linear relationships', 'A linear model has a constant rate of change over its stated domain. A straight line drawn through two points is not enough evidence that the relationship remains linear outside the observed range.', 'State the domain', 'Test the prediction'),
 ('Sampling and bias', 'A large sample can still be biased when the selection procedure systematically excludes a group. Describe how observations were collected and which population the conclusion is intended to represent.', 'Describe the sample', 'Check for exclusions'),
 ('Energy transfer', 'Energy may move between objects or change form while the total remains conserved in a closed system. Draw the system boundary first, then identify the transfers that cross it.', 'Draw the boundary', 'Account for transfers'),
 ('Pressure and area', 'Pressure compares a force with the area over which it acts. The same force produces a larger pressure when the contact area is smaller. Use consistent units when comparing measurements.', 'Measure the area', 'Compare the forces'),
 ('Waves and frequency', 'Frequency counts complete cycles in a unit of time. Wavelength measures the distance between corresponding points on neighboring cycles. A change in the medium can change the wave speed.', 'Count complete cycles', 'Describe the medium'),
 ('Habitats and adaptation', 'An adaptation can improve survival in a particular environment without being useful everywhere. A change in habitat may alter which traits are beneficial. Compare several sources of evidence.', 'Describe the habitat', 'Compare the traits'),
 ('Resources and recycling', 'Recycling can reduce the need for some raw materials, but collection and processing also consume resources. Evaluate the complete process instead of considering only its final product.', 'Trace the process', 'Compare resource use'),
 ('Algorithms and termination', 'An algorithm should specify a finite sequence of unambiguous steps. For a loop, explain what changes on every iteration and why the stopping condition will eventually become true.', 'State the invariant', 'Explain termination'),
 ('Data structures', 'A suitable data structure depends on the operations that a program performs most often. Fast lookup, ordered traversal, and compact storage are different requirements and may involve tradeoffs.', 'List the operations', 'Measure the tradeoffs'),
 ('Reliable storage', 'A backup is useful only when it can be restored. Keep the original data separate from the recovery destination. Verify both the restored records and the application behavior after recovery.', 'Keep an isolated copy', 'Verify the recovery'),
 ('Access control', 'A resource identifier is not evidence of permission. Check the current authenticated user and the resource membership on every request. A search index must not become a source of authorization.', 'Check each resource', 'Use trusted identity'),
 ('Versioned documents', 'A quotation should identify the exact document version and location from which it was taken. Later corrections must not silently change the meaning of an earlier citation.', 'Preserve the version', 'Verify the quotation'),
 ('Testing failures', 'A useful test checks a specific behavior and fails when that behavior is broken. Keep the original failure evidence, repair the cause, and run the relevant checks again before reporting success.', 'Keep the failed case', 'Test the repaired path'),
 ('Historical sources', 'A historical source reflects the context in which it was produced. Compare its date, purpose, and audience with other evidence. Distinguish a direct record from a later interpretation.', 'Compare the context', 'Identify the audience'),
 ('Clear explanations', 'A clear explanation states the main claim and connects each supporting fact to it. Define unfamiliar terms when they first appear. A diagram should clarify a relationship rather than replace evidence.', 'State the main claim', 'Connect the evidence'),
 ('Project planning', 'Break a project into concrete deliverables with observable acceptance conditions. Work on independent preparation while a long check is running. Keep dependencies explicit and record actual results.', 'Define the deliverable', 'Track the dependencies'),
 ('Responsible computing', 'Software should collect only the information required for its stated purpose. Test logs must not expose private documents or credentials. Use synthetic examples when evaluating a new feature.', 'Minimize collected data', 'Use synthetic examples'),
]
ROT = [
 ('旋转图像与坐标', '扫描页方向改变以后，识别结果需要映射回原始输入页。页码和位置都要核对，不能直接把矫正后的坐标当作原图坐标。', '映射原始坐标', '核对页面方向', 'zh'),
 ('测量误差', '测量值与真实值之间可能存在误差。重复测量有助于观察波动，但不能自动消除仪器的系统偏差。报告结果时应说明精度。', '记录测量精度', '检查系统偏差', 'zh'),
 ('力与平衡', '物体保持静止时，合力为零并不代表每个力都为零。分析受力需要明确研究对象，再逐个标注作用力的方向。', '明确研究对象', '分别标注作用力', 'zh'),
 ('阅读与结构', '段落中的主题句帮助读者理解核心观点。举例和比较应当支持这个观点。阅读时可以标注转折词，检查前后关系是否一致。', '标注主题句', '核对转折关系', 'zh'),
 ('可重复的实验', '别人能够按照记录重复实验，是判断方法是否清楚的重要依据。记录设备、步骤和条件，不能只保留最终成功的图片。', '记录设备条件', '保存完整步骤', 'zh'),
 ('Coordinates and rotation', 'Rotating a scanned page changes its display coordinates. Map recognition boxes back to the original input before comparing locations. Preserve the reading order separately from the pixel orientation.', 'Map the coordinates', 'Preserve reading order', 'en'),
 ('Graphs and connectivity', 'A graph contains vertices and edges that describe a chosen relationship. A path connects a sequence of adjacent vertices. The meaning of an edge must be defined before interpreting the diagram.', 'Define the relation', 'Follow a valid path', 'en'),
 ('Conservation and boundaries', 'A balance calculation depends on a clearly defined system. Count material that enters or leaves through its boundary. An apparent loss may result from observing only part of the system.', 'Define the system', 'Count each transfer', 'en'),
 ('Evidence and uncertainty', 'An estimate should describe the observations on which it depends and the uncertainty that remains. More decimal places do not necessarily mean greater accuracy. Explain the limits of the method.', 'Report uncertainty', 'Explain the limits', 'en'),
 ('Communication protocols', 'A protocol defines the sequence and meaning of messages exchanged by participants. A missing acknowledgment does not prove that a request was never processed. Recovery rules must handle this ambiguity.', 'Define each message', 'Handle lost replies', 'en'),
]


def digest(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def rotate_box(b, angle):
    x1, y1, x2, y2 = b
    return {0: b, 90: (H-y2, x1, H-y1, x2),
            180: (W-x2, H-y2, W-x1, H-y1),
            270: (y1, W-x2, y2, W-x1)}[angle]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--font', required=True)
    ap.add_argument('--license', required=True)
    ap.add_argument('--out', required=True)
    args = ap.parse_args()
    assert PIL.__version__ == '13.0.0' and reportlab.Version == '4.4.9'
    assert digest(Path(args.font)) == FONT_SHA
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    for name in ('blind', 'development-negative', 'development-positive', 'pdf', 'licenses'):
        (out/name).mkdir(exist_ok=True)
    shutil.copyfile(args.license, out/'licenses/NotoSansSC-OFL.txt')

    def font(size, weight):
        f = ImageFont.truetype(args.font, size)
        f.set_variation_by_axes([weight])
        return f

    def make(identifier, group, language, content, index, angle=0, degradation=None):
        title, paragraph, action1, action2 = content
        split = 'blind' if group in ('clear_zh', 'clear_en', 'rotated') else group
        im = Image.new('RGB', (W, H), 'white')
        draw = ImageDraw.Draw(im)
        weight = [400, 500, 600][index % 3]
        body = font([36, 38, 40][index % 3], weight)
        heading = font(51, 700)
        cursor, lines, fragments = 140, [], []

        def line(text, f=body, gap=27, selected=False):
            nonlocal cursor
            b = draw.textbbox((120, cursor), text, font=f, anchor='lt')
            assert b[2] <= W-100 and b[3] < H-110, (identifier, text, b)
            draw.text((120, cursor), text, font=f, fill=0, anchor='lt')
            start = sum(len(t)+1 for t in lines)
            fragments.append({'text': text, 'char_start': start, 'char_end': start+len(text),
                              'pixel_bbox': list(b), 'selected': selected})
            lines.append(text)
            cursor = b[3] + gap

        def paragraph_lines(text):
            # English word wrapping avoids inventing spaces inside words.
            units = text.split(' ') if language == 'en' else list(text)
            sep = ' ' if language == 'en' else ''
            current = ''
            for part in units:
                candidate = current + (sep if current else '') + part
                if draw.textlength(candidate, font=body) > W-240 and current:
                    line(current)
                    current = part
                else:
                    current = candidate
            if current:
                line(current)

        line(title, heading, gap=55)
        line('课堂阅读与练习' if language == 'zh' else 'Classroom reading and practice', gap=36)
        paragraph_lines(paragraph)
        cursor += 35
        line(action1, selected=True)
        line(action2, selected=True)
        cursor += 32
        if language == 'zh':
            paragraph_lines(f'练习要求：用自己的话说明以上方法，并给出一个可以核对的例子。记录编号为{index+1:02d}，先完成观察，再写结论。')
            line('检查项       已记录       待复核', gap=23)
            line('资料来源     完整         位置与日期', gap=23)
            line('推理过程     保留         前提与边界', gap=23)
            line(f'课堂练习第{index+1:02d}页。全部文字为自制测试材料。', gap=20)
        else:
            paragraph_lines(f'Practice {index+1:02d}: explain the method in your own words and give one example that another reader can verify. Record observations before writing a conclusion.')
            line('Check item       Recorded       Review next', gap=23)
            line('Source           Complete       Place and date', gap=23)
            line('Reasoning        Retained       Limits and assumptions', gap=23)
            paragraph_lines('This page is self-authored test material, not a real student record.')
        if degradation:
            if degradation['kind'] == 'blur':
                im = im.filter(ImageFilter.GaussianBlur(degradation['radius']))
                im = Image.blend(Image.new('RGB', im.size, 'white'), im, .30)
            else:
                d = ImageDraw.Draw(im)
                for y in range(180, min(int(cursor), H-100), 88):
                    d.rectangle((110, y, W-90, y+48), fill='white')
        if angle:
            im = im.transpose({90: Image.Transpose.ROTATE_270, 180: Image.Transpose.ROTATE_180,
                               270: Image.Transpose.ROTATE_90}[angle])
        extension = 'jpg' if index % 4 == 1 else 'png'
        path = out/split/(identifier+'.'+extension)
        if extension == 'jpg':
            im.save(path, quality=92, subsampling=0, optimize=False, dpi=(DPI, DPI))
        else:
            im.save(path, compress_level=9, dpi=(DPI, DPI))
        boxes = []
        for f in fragments:
            if f['selected']:
                b = rotate_box(f['pixel_bbox'], angle)
                boxes.append({'id': identifier+'-b'+str(len(boxes)+1), 'text': f['text'],
                              'char_start': f['char_start'], 'char_end': f['char_end'],
                              'bbox': [round(b[0]/im.width, 8), round(b[1]/im.height, 8),
                                       round(b[2]/im.width, 8), round(b[3]/im.height, 8)]})
        return {'id': identifier, 'split': split, 'group': group, 'language': language,
                'file': path.relative_to(out).as_posix(), 'sha256': digest(path),
                'bytes': path.stat().st_size, 'width': im.width, 'height': im.height,
                'dpi': DPI, 'clockwise_rotation': angle, 'font_weight': weight,
                'gold': '\n'.join(lines), 'boxes': boxes if split == 'blind' else [],
                'expected_class': 'needs_review_or_failed' if degradation else 'recognizable',
                'degradation': degradation,
                'origin': 'self-authored synthetic classroom text; no personal data'}

    pages = []
    for lang, cases in [('zh', CN), ('en', EN)]:
        for i, text in enumerate(cases):
            pages.append(make(f'{lang}-{i+1:02d}', 'clear_'+lang, lang, text, i))
    for i, (*text, lang) in enumerate(ROT):
        pages.append(make(f'rot-{i+1:02d}', 'rotated', lang, text, 40+i, [90, 180, 270][i%3]))
    # Disjoint wording: these pages are available for tuning and never score as blind data.
    for i in range(10):
        lang = 'zh' if i < 5 else 'en'
        text = (f'低质量开发材料{i+1:02d}', '这份页面用于检查模糊与遮挡提示。无法可靠辨认的文字必须交由教师核对，不应作为已经确认的资料发布。请保留原始页面，说明需要重新扫描的位置。', '检查图像质量', '提示人工复核') if lang == 'zh' else (
            f'Low quality development page {i+1:02d}', 'This development image intentionally hides or blurs important content. Unreadable text requires a visible review state. Keep the original image and ask for a clearer scan instead of publishing an invented transcription.', 'Inspect image quality', 'Request human review')
        degradation = {'kind': 'blur', 'radius': 8+i%5} if i % 2 == 0 else {'kind': 'occlusion', 'stripe_pixels': 48}
        pages.append(make(f'negative-{i+1:02d}', 'development-negative', lang, text, 60+i, degradation=degradation))
    positives = [
        ('开发样本：水循环', '水蒸发形成水汽，遇冷后可以凝结。这个样本只用于独立开发与工具验证，不进入正式盲测分数。观察时记录现象发生的条件。', '观察凝结过程', '记录环境条件', 'zh'),
        ('开发样本：植物生长', '植物生长需要适宜的光照、水分和温度。比较两盆植物时，应说明各自环境的差异。本页只供开发选型使用。', '记录光照条件', '比较生长记录', 'zh'),
        ('Development sample: seasons', 'The seasons repeat as the position of Earth changes during its orbit. This independent page is available for development and tool selection only. It is excluded from the frozen blind score.', 'Observe the seasons', 'Record the position', 'en'),
        ('Development sample: evaporation', 'Water can evaporate from an open container. Record the temperature and exposed area when comparing observations. This independent example never contributes to the blind evaluation.', 'Measure exposed area', 'Record temperature', 'en'),
    ]
    for i, (*text, lang) in enumerate(positives):
        pages.append(make(f'dev-{i+1:02d}', 'development-positive', lang, text, 80+i, [0, 90, 0, 180][i]))
    pdfs = []
    for split in ('blind', 'development-negative', 'development-positive'):
        selected = [p for p in pages if p['split'] == split]
        path = out/'pdf'/(split+'.pdf')
        c = canvas.Canvas(str(path), invariant=1, pageCompression=1)
        c.setTitle('CampusClawPlus OCR '+split+' v1')
        c.setAuthor('CampusClawPlus synthetic fixtures')
        for n, p in enumerate(selected, 1):
            width, height = p['width']*72/DPI, p['height']*72/DPI
            c.setPageSize((width, height))
            # ImageReader keys PDF image objects by bytes, not the output path.
            c.drawImage(ImageReader(str(out/p['file'])), 0, 0, width=width, height=height)
            c.showPage()
            p['pdf_page'] = n
            p['pdf_file'] = path.relative_to(out).as_posix()
        c.save()
        pdfs.append({'file': path.relative_to(out).as_posix(), 'split': split, 'pages': len(selected),
                     'sha256': digest(path), 'bytes': path.stat().st_size, 'text_layer': False})
    manifest = {'version': 1, 'scenario': ['DI-SC-16', 'DI-SC-17'],
                'purpose': 'frozen inputs and gold; OCR quality NOT measured',
                'coordinate_frame': 'original input raster before OCR orientation correction; top-left normalized xyxy',
                'score_normalization': 'Unicode NFC, White_Space collapse, trim; keep case and punctuation',
                'thresholds': {'clear_micro_cer': .05, 'rotated_micro_cer': .10,
                               'bbox_iou': .50, 'bbox_match_rate': .95, 'page_accuracy': 1,
                               'negative_review_minimum': 9},
                'blind_policy': 'No blind OCR invocation or parameter selection before lock approval; all tuning uses development only.',
                'authoring': {'pillow': PIL.__version__, 'reportlab': reportlab.Version,
                              'font_sha256': FONT_SHA, 'font_license': 'SIL-OFL-1.1',
                              'font_source': 'https://raw.githubusercontent.com/google/fonts/a85815a42757630ce188fdad368c2dfc444d4773/ofl/notosanssc/NotoSansSC%5Bwght%5D.ttf',
                              'generator_sha256': digest(Path(__file__))},
                'pages': pages, 'pdfs': pdfs}
    (out/'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2)+'\n', encoding='utf-8', newline='\n')
    all_files = sorted(p for p in out.rglob('*') if p.is_file() and p.name != 'corpus-lock.json')
    lock = {'version': 1, 'quality_measured': False,
            'sha256': {p.relative_to(out).as_posix(): digest(p) for p in all_files}}
    (out/'corpus-lock.json').write_text(json.dumps(lock, ensure_ascii=False, indent=2)+'\n', encoding='utf-8', newline='\n')
    print(json.dumps({'pages': len(pages), 'blind': 50, 'negative': 10, 'development_positive': 4,
                      'blind_boxes': sum(len(p['boxes']) for p in pages if p['split']=='blind'),
                      'pdfs': len(pdfs), 'quality_measured': False}))


if __name__ == '__main__':
    main()
