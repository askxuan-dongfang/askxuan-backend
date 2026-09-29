import json,copy
from pathlib import Path
root=Path(__file__).resolve().parents[1]
def f(key,label,type='text',required=True,**kw):return dict(key=key,label=label,type=type,required=required,**kw)
def select(key,label,options,default=None,required=True):
 d=f(key,label,'select',required,options=[dict(value=v,label=l) for v,l in options]);
 if default is not None:d['defaultValue']=default
 return d
def num(key,label,lo,hi,required=True,integer=True):return f(key,label,'number',required,min=lo,max=hi,validation='integer' if integer else 'number')
def when(field,key,value):return dict(field,required=False,visibleWhen=dict(key=key,value=value),requiredWhen=dict(key=key,value=value))
birth=[select('calendarType','历法',[('solar','公历'),('lunar','农历')],'solar'),f('birthDate','出生日期','date'),f('birthTime','出生时间','time'),select('gender','性别',[('male','男'),('female','女')])]
place=f('birthplace','出生地',required=False,helpText='城市和区县即可；地点校正以工具实际返回状态为准。')
event=f('eventTime','起局时间','datetime',helpText='按北京时间（UTC+8）填写。')
palaces=[(x,x) for x in ['命宫','兄弟','夫妻','子女','财帛','疾厄','迁移','仆役','官禄','田宅','福德','父母']]
rows=[]
def add(code,name,desc,fields,keywords,group):rows.append(dict(code=code,name=name,description=desc,inputSchema=dict(fields=copy.deepcopy(fields)),keywords=keywords,group=group))
add('bazi','八字命盘','读取四柱、十神与命盘关系；涉及大运流年时继续调用八字大运。',birth+[place],['八字','四柱','十神'],'命盘与运限')
add('bazi_dayun','八字大运','计算起运、大运、小运与流年链路，解释实际返回的数据。',birth,['八字大运','大运','起运','小运'],'命盘与运限')
add('bazi_pillars_resolve','四柱反推','根据已知四柱查找出生时间候选；候选结果不作为用户已确认的出生资料。',[f(k,l,validation='pillar',helpText='填写一个有效干支，例如甲子。') for k,l in [('yearPillar','年柱'),('monthPillar','月柱'),('dayPillar','日柱'),('hourPillar','时柱')]],['四柱反推','反推出生'],'命盘与运限')
add('ziwei','紫微命盘','查看紫微十二宫位与星曜；运限和四化飞星使用对应工具。',birth+[place],['紫微','斗数'],'命盘与运限')
add('ziwei_horoscope','紫微运限','按指定日期读取大限、小限、流年、流月、流日与流时。',birth+[place,f('targetDate','要查看的日期','date'),num('targetTimeIndex','流时时段序号',0,12,False)],['紫微运限','紫微流年','大限','流月','流时'],'命盘与运限')
add('ziwei_flying_star','紫微飞星','查看指定宫位的四化落宫、自化、飞入及三方四正。',birth+[place,select('queryType','查询内容',[('mutagedPlaces','四化落宫'),('selfMutaged','自化'),('surroundedPalaces','三方四正'),('fliesTo','宫位飞入')],'mutagedPlaces'),select('palace','查询宫位',palaces,'命宫'),when(select('toPalace','目标宫位',palaces),'queryType','fliesTo')],['紫微飞星','四化','自化','三方四正'],'命盘与运限')
add('astrology','西方占星','计算本命盘与指定时刻的流运盘；宫位计算使用用户确认的坐标。',[f('astroBirthDate','当地公历出生日期','date',helpText='填写出生地当地公历日期；不要直接复用农历日期。'),f('astroBirthTime','当地出生时间','time',helpText='填写出生地当地时间；工具根据你确认的经纬度确定时区。'),num('latitude','出生地纬度',-90,90,integer=False),num('longitude','出生地经度',-180,180,integer=False),f('transitDateTime','流运时刻','datetime',helpText='按北京时间填写；地点坐标必须来自你确认的信息。')],['西方占星','星盘','本命盘','流运盘'],'命盘与运限')
add('qimen','奇门遁甲','按时间排出奇门九宫、九星、八门和八神。',[event,f('location','所在地',required=False)],['奇门','遁甲'],'问事与起卦')
add('tarot','塔罗牌','按选择的牌阵抽牌并解释牌面。',[select('spread','牌阵',[(x,y) for x,y in [('single','单牌'),('three','三牌阵'),('love','爱情牌阵'),('decision','抉择牌阵'),('celtic-cross','凯尔特十字'),('horseshoe','马蹄牌阵'),('mind-body-spirit','身心灵'),('situation','处境牌阵'),('yes-no','是否牌阵')]],'single')],['塔罗','抽牌'],'问事与起卦')
add('liuyao','六爻排卦','通过自动、时间、数字或指定卦起卦，读取六爻盘面。',[select('method','起卦方式',[('auto','自动起卦'),('time','时间起卦'),('number','数字起卦'),('select','指定卦')],'auto'),when(f('numbers','起卦数字',validation='divination-numbers'),'method','number'),when(f('hexagramName','本卦卦名或卦码'),'method','select'),dict(when(f('changedHexagramName','变卦卦名或卦码'),'method','select'),requiredWhen=None),select('yongShenTarget','关注事项',[(x,x) for x in ['官鬼','妻财','子孙','父母','兄弟']]),when(event,'method','time')],['六爻','排卦'],'问事与起卦')
meihua=[select('meihuaMethod','起卦方式',[('time','时间起卦'),('number_pair','两个数字'),('number_triplet','三个数字'),('text_split','字占'),('count_with_time','物数或声数'),('measure','尺寸起卦'),('classifier_pair','类象起卦'),('select','指定卦')],'time'),event]
for k,l,typ in [('pairNumbers','两个数字','pair-numbers'),('tripleNumbers','三个数字','triple-numbers')]:meihua.append(when(f(k,l,validation=typ),'meihuaMethod','number_pair' if k=='pairNumbers' else 'number_triplet'))
meihua += [when(f('divinationText','字占文本'),'meihuaMethod','text_split'),when(num('count','数量',1,999999),'meihuaMethod','count_with_time'),when(select('countCategory','数量来源',[('item','物数'),('sound','声数')]),'meihuaMethod','count_with_time'),when(select('measureKind','量法',[('丈尺','丈尺'),('尺寸','尺寸')]),'meihuaMethod','measure'),when(num('majorValue','大单位数值',0,999999),'meihuaMethod','measure'),when(num('minorValue','小单位数值',0,999999),'meihuaMethod','measure'),when(f('upperCue','上卦类象'),'meihuaMethod','classifier_pair'),when(f('lowerCue','下卦类象'),'meihuaMethod','classifier_pair'),when(f('hexagramName','本卦卦名或卦码'),'meihuaMethod','select'),when(num('movingLine','动爻',1,6),'meihuaMethod','select')]
add('meihua','梅花易数','按时间、数字、字占、物数、尺寸或类象起卦，读取体用关系。',meihua,['梅花','梅花易数'],'问事与起卦')
add('taiyi','太乙九星','按选定尺度查看太乙九星时空底盘。',[select('taiyiMode','观测尺度',[('year','年'),('month','月'),('day','日'),('hour','时'),('minute','分钟')],'day'),event],['太乙','九星观测'],'问事与起卦')
add('daliuren','大六壬','按指定时间起课，读取天地盘、四课、三传和神将。',[event],['大六壬'],'问事与起卦')
add('xiaoliuren','小六壬','按农历月日与明确的时辰起课。',[num('lunarMonth','农历月份',1,12),num('lunarDay','农历日期',1,30),select('hourIndex','时辰',[(str(i+1),s+'时') for i,s in enumerate('子丑寅卯辰巳午未申酉戌亥')])],['小六壬'],'问事与起卦')
add('almanac','黄历查询','查看指定日期的黄历、宜忌和时辰信息。',[f('targetDate','查询日期','date'),select('dayMaster','日主天干（已知时填写）',[(x,x) for x in '甲乙丙丁戊己庚辛壬癸'],required=False)],['黄历','宜忌'],'生活参考')
p=root/'services/infrastructure/ai-service/internal/agent/tool_catalog.json';p.write_text(json.dumps(rows,ensure_ascii=False,indent=2)+'\n')
def q(v):return "CONVERT(0x"+v.encode().hex()+" USING utf8mb4)"
lines=['-- Reviewed taibu tool catalog. No schema or financial data changes.','USE askxuan_ai;','START TRANSACTION;']
cols=['code','category','name','version','description','icon','source_type','source_ref','prompt_template','input_schema','routing_keywords','capabilities','tool_config','risk_level','sort_order','status']
for i,r in enumerate(rows):
 vals=[r['code'],'divination',r['name'],'3.0.0',r['description'],'','reviewed_skill','askxuan/taibu-reviewed@20260929','你是'+r['name']+'助手。'+r['description']+'仅解释实际工具结果；缺少资料时补问，不猜测事实。不替代医疗、法律或投资意见。',json.dumps(r['inputSchema'],ensure_ascii=False),json.dumps(r['keywords'],ensure_ascii=False),json.dumps(['chat','stream','structured_input','mcp','auto_route','reasoning_status']),json.dumps(dict(enabled=True,server='taibu',tool=r['code'])),'medium',10+i*10,'enabled']
 # Existing operator enable/disable status is retained; new entries are enabled.
 updates=','.join(c+'=VALUES('+c+')' for c in cols if c not in ['code','status'])
 lines.append('INSERT INTO ai_skill ('+','.join(cols)+') VALUES ('+','.join(str(v) if isinstance(v,int) else q(v) for v in vals)+') ON DUPLICATE KEY UPDATE '+updates+';')
lines+=['COMMIT;']
(root/'scripts/db/20260929_ai_taibu_tools.sql').write_text('\n'.join(lines)+'\n')
print(len(rows),'tools')
